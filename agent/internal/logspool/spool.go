package logspool

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrSpoolFull = errors.New("log spool quota exceeded")

const (
	maxEventsPerBatch = 32
	maxEventBodyBytes = 16 << 10
)

type Event struct {
	ID string `json:"event_id"`
	EventTimeUnixNanos int64 `json:"event_time_unix_nanos"`
	ObservedAtUnixNanos int64 `json:"observed_at_unix_nanos"`
	Source string `json:"source"`
	Severity string `json:"severity"`
	Body string `json:"body"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Batch struct {
	ID string `json:"batch_id"`
	CollectedAtUnixNanos int64 `json:"collected_at_unix_nanos"`
	SchemaVersion uint32 `json:"schema_version"`
	Cursor string `json:"cursor"`
	Events []Event `json:"events"`
}

type Store struct {
	mu sync.Mutex
	path string
	cursorPath string
	maxBytes int64
}

func Open(directory string, maxBytes int64) (*Store, error) {
	if directory == "" || maxBytes < 1<<20 {
		return nil, errors.New("log spool directory and a quota of at least 1MiB are required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create log spool directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure log spool directory: %w", err)
	}
	path := filepath.Join(directory, "queue.jsonl")
	for _, candidate := range []string{path, filepath.Join(directory, "cursor")} {
		if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("log spool files must not be symbolic links")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect log spool file: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("initialize log spool queue: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("secure log spool queue: %w", err)
	}
	store := &Store{path: path, cursorPath: filepath.Join(directory, "cursor"), maxBytes: maxBytes}
	if err := store.recoverTrailingRecord(); err != nil {
		return nil, err
	}
	return store, nil
}

func NewBatch(events []Event, cursor string, collectedAtUnixNanos int64) (Batch, error) {
	if len(events) == 0 || len(events) > maxEventsPerBatch || cursor == "" || len(cursor) > 1024 {
		return Batch{}, errors.New("log batch event count or cursor is invalid")
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return Batch{}, fmt.Errorf("generate log batch ID: %w", err)
	}
	batch := Batch{
		ID: hex.EncodeToString(randomID[:]), CollectedAtUnixNanos: collectedAtUnixNanos,
		SchemaVersion: 1, Cursor: cursor, Events: events,
	}
	if err := validateBatch(batch); err != nil {
		return Batch{}, err
	}
	return batch, nil
}

func (s *Store) Cursor() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.cursorPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read acknowledged journal cursor: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Store) Append(batch Batch) error {
	if err := validateBatch(batch); err != nil {
		return err
	}
	encoded, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("encode log spool batch: %w", err)
	}
	encoded = append(encoded, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("stat log spool queue: %w", err)
	}
	if info.Size()+int64(len(encoded)) > s.maxBytes {
		return ErrSpoolFull
	}
	file, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log spool queue: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return fmt.Errorf("append log spool batch: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync log spool batch: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log spool queue: %w", err)
	}
	return nil
}

func (s *Store) Peek(limit int) ([]Batch, error) {
	if limit <= 0 {
		return nil, errors.New("log spool read limit must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked(limit)
}

// Ack only accepts the head batch. The cursor is advanced after queue
// compaction; if cursor persistence fails, the journal may be replayed, but
// stable event IDs make that replay idempotent at the server.
func (s *Store) Ack(batchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	batches, err := s.readLocked(0)
	if err != nil {
		return err
	}
	if len(batches) == 0 {
		return errors.New("cannot acknowledge an empty log spool")
	}
	if batches[0].ID != batchID {
		return errors.New("log spool acknowledgements must follow queue order")
	}

	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".log-queue-*.tmp")
	if err != nil {
		return fmt.Errorf("create log spool compaction file: %w", err)
	}
	tempName := temporary.Name()
	defer os.Remove(tempName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure log spool compaction file: %w", err)
	}
	writer := bufio.NewWriter(temporary)
	for _, batch := range batches[1:] {
		encoded, marshalErr := json.Marshal(batch)
		if marshalErr != nil {
			_ = temporary.Close()
			return fmt.Errorf("encode retained log batch: %w", marshalErr)
		}
		if _, writeErr := writer.Write(append(encoded, '\n')); writeErr != nil {
			_ = temporary.Close()
			return fmt.Errorf("write compacted log spool: %w", writeErr)
		}
	}
	if err := writer.Flush(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush compacted log spool: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync compacted log spool: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close compacted log spool: %w", err)
	}
	if err := replaceFile(tempName, s.path); err != nil {
		return fmt.Errorf("replace log spool queue: %w", err)
	}
	if err := s.writeCursorLocked(batches[0].Cursor); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(s.path))
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func (s *Store) writeCursorLocked(cursor string) error {
	temporary, err := os.CreateTemp(filepath.Dir(s.cursorPath), ".cursor-*.tmp")
	if err != nil {
		return fmt.Errorf("create journal cursor checkpoint: %w", err)
	}
	tempName := temporary.Name()
	defer os.Remove(tempName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(cursor + "\n"); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write journal cursor checkpoint: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync journal cursor checkpoint: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceFile(tempName, s.cursorPath); err != nil {
		return fmt.Errorf("replace journal cursor checkpoint: %w", err)
	}
	return nil
}

func (s *Store) Stats() (int64, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Stat(s.path)
	if err != nil {
		return 0, 0, fmt.Errorf("stat log spool queue: %w", err)
	}
	batches, err := s.readLocked(0)
	if err != nil {
		return info.Size(), 0, err
	}
	return info.Size(), len(batches), nil
}

func (s *Store) recoverTrailingRecord() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read log spool queue for recovery: %w", err)
	}
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return nil
	}
	lastNewline := bytes.LastIndexByte(data, '\n')
	prefixLength := lastNewline + 1
	tail := bytes.TrimSpace(data[prefixLength:])
	var batch Batch
	complete := len(tail) > 0 && json.Unmarshal(tail, &batch) == nil && validateBatch(batch) == nil
	file, err := os.OpenFile(s.path, os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log spool for recovery: %w", err)
	}
	if complete {
		if _, err := file.WriteAt([]byte("\n"), int64(len(data))); err != nil {
			_ = file.Close()
			return fmt.Errorf("finish complete trailing log record: %w", err)
		}
	} else if err := file.Truncate(int64(prefixLength)); err != nil {
		_ = file.Close()
		return fmt.Errorf("truncate incomplete log record: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync recovered log spool: %w", err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = s.readLocked(0)
	return err
}

func (s *Store) readLocked(limit int) ([]Batch, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("open log spool for reading: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	batches := make([]Batch, 0)
	for scanner.Scan() {
		var batch Batch
		if err := json.Unmarshal(scanner.Bytes(), &batch); err != nil {
			return nil, fmt.Errorf("decode log spool record: %w", err)
		}
		if err := validateBatch(batch); err != nil {
			return nil, fmt.Errorf("validate log spool record: %w", err)
		}
		batches = append(batches, batch)
		if limit > 0 && len(batches) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read log spool queue: %w", err)
	}
	return batches, nil
}

func validateBatch(batch Batch) error {
	if len(batch.ID) != 32 || strings.ToLower(batch.ID) != batch.ID {
		return errors.New("log batch ID must be 32 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(batch.ID); err != nil {
		return errors.New("log batch ID must be hexadecimal")
	}
	if batch.CollectedAtUnixNanos <= 0 || batch.SchemaVersion != 1 ||
		len(batch.Events) == 0 || len(batch.Events) > maxEventsPerBatch ||
		batch.Cursor == "" || len(batch.Cursor) > 1024 {
		return errors.New("log batch metadata is invalid")
	}
	for _, event := range batch.Events {
		if len(event.ID) != 64 || len(event.Source) == 0 || len(event.Source) > 128 ||
			len(event.Body) == 0 || len(event.Body) > maxEventBodyBytes ||
			event.EventTimeUnixNanos <= 0 || event.ObservedAtUnixNanos <= 0 ||
			!validSeverity(event.Severity) || len(event.Attributes) > 16 {
			return errors.New("log event is invalid")
		}
		if _, err := hex.DecodeString(event.ID); err != nil || strings.ToLower(event.ID) != event.ID {
			return errors.New("log event ID must be lowercase hexadecimal")
		}
		for key, value := range event.Attributes {
			if key == "" || len(key) > 64 || len(value) > 256 ||
				strings.ContainsAny(value, "\r\n\x00") {
				return errors.New("log event attribute is invalid")
			}
		}
	}
	return nil
}

func validSeverity(value string) bool {
	switch value {
	case "emergency", "alert", "critical", "error", "warning", "notice", "info", "debug", "unknown":
		return true
	default:
		return false
	}
}
