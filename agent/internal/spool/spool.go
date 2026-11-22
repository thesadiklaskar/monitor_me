package spool

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/WorkRCS/monitor_me/agent/internal/collector"
)

var ErrSpoolFull = errors.New("agent spool quota exceeded")

const maxBatchSamples = 4096

type Batch struct {
	ID                   string             `json:"id"`
	CollectedAtUnixNanos int64              `json:"collected_at_unix_nanos"`
	SchemaVersion        uint32             `json:"schema_version"`
	Samples              []collector.Sample `json:"samples"`
}

type Store struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
}

func Open(directory string, maxBytes int64) (*Store, error) {
	if directory == "" || maxBytes < 1<<20 {
		return nil, errors.New("spool directory and a quota of at least 1MiB are required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create spool directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure spool directory permissions: %w", err)
	}
	path := filepath.Join(directory, "queue.jsonl")
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("spool queue must not be a symbolic link")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect spool queue: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("initialize spool queue: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close spool queue: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("secure spool queue permissions: %w", err)
	}
	return &Store{path: path, maxBytes: maxBytes}, nil
}

func NewBatch(samples []collector.Sample, collectedAtUnixNanos int64) (Batch, error) {
	if len(samples) == 0 || len(samples) > maxBatchSamples {
		return Batch{}, errors.New("telemetry batch sample count is outside the allowed range")
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return Batch{}, fmt.Errorf("generate telemetry batch id: %w", err)
	}
	batch := Batch{
		ID: hex.EncodeToString(randomID[:]), CollectedAtUnixNanos: collectedAtUnixNanos,
		SchemaVersion: 1, Samples: samples,
	}
	if err := validateBatch(batch); err != nil {
		return Batch{}, err
	}
	return batch, nil
}

func (s *Store) Append(batch Batch) error {
	if err := validateBatch(batch); err != nil {
		return err
	}
	encoded, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("encode telemetry batch: %w", err)
	}
	encoded = append(encoded, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("stat spool queue: %w", err)
	}
	if info.Size()+int64(len(encoded)) > s.maxBytes {
		return ErrSpoolFull
	}
	file, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open spool queue: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(encoded); err != nil {
		return fmt.Errorf("append spool batch: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync spool batch: %w", err)
	}
	return nil
}

func (s *Store) Peek(limit int) ([]Batch, error) {
	if limit <= 0 {
		return nil, errors.New("spool read limit must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked(limit)
}

func (s *Store) Ack(batchIDs []string) error {
	if len(batchIDs) == 0 {
		return nil
	}
	acknowledged := make(map[string]struct{}, len(batchIDs))
	for _, id := range batchIDs {
		acknowledged[id] = struct{}{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	batches, err := s.readLocked(0)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".queue-*.tmp")
	if err != nil {
		return fmt.Errorf("create spool compaction file: %w", err)
	}
	tempName := temporary.Name()
	defer os.Remove(tempName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure spool compaction file: %w", err)
	}
	writer := bufio.NewWriter(temporary)
	for _, batch := range batches {
		if _, remove := acknowledged[batch.ID]; remove {
			continue
		}
		encoded, marshalErr := json.Marshal(batch)
		if marshalErr != nil {
			temporary.Close()
			return fmt.Errorf("encode retained spool batch: %w", marshalErr)
		}
		if _, writeErr := writer.Write(append(encoded, '\n')); writeErr != nil {
			temporary.Close()
			return fmt.Errorf("write compacted spool queue: %w", writeErr)
		}
	}
	if err := writer.Flush(); err != nil {
		temporary.Close()
		return fmt.Errorf("flush compacted spool queue: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync compacted spool queue: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close compacted spool queue: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("replace compacted spool queue: %w", err)
	}
	directory, err := os.Open(filepath.Dir(s.path))
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func (s *Store) Stats() (bytes int64, batches int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Stat(s.path)
	if err != nil {
		return 0, 0, fmt.Errorf("stat spool queue: %w", err)
	}
	values, err := s.readLocked(0)
	if err != nil {
		return info.Size(), 0, err
	}
	return info.Size(), len(values), nil
}

func (s *Store) readLocked(limit int) ([]Batch, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("open spool queue for reading: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	batches := make([]Batch, 0)
	for scanner.Scan() {
		var batch Batch
		if err := json.Unmarshal(scanner.Bytes(), &batch); err != nil {
			return nil, fmt.Errorf("decode spool WAL record: %w", err)
		}
		if err := validateBatch(batch); err != nil {
			return nil, fmt.Errorf("validate spool WAL record: %w", err)
		}
		batches = append(batches, batch)
		if limit > 0 && len(batches) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read spool WAL: %w", err)
	}
	return batches, nil
}

func validateBatch(batch Batch) error {
	if len(batch.ID) != 32 || batch.CollectedAtUnixNanos <= 0 || batch.SchemaVersion != 1 ||
		len(batch.Samples) == 0 || len(batch.Samples) > maxBatchSamples {
		return errors.New("telemetry batch metadata is invalid")
	}
	for _, sample := range batch.Samples {
		if sample.Name == "" || len(sample.Name) > 128 || sample.Unit == "" || len(sample.Unit) > 64 ||
			sample.ObservedAtUnixNanos <= 0 || math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) ||
			len(sample.Labels) > 32 {
			return errors.New("telemetry sample is invalid")
		}
		for key, value := range sample.Labels {
			if key == "" || len(key) > 64 || len(value) > 256 {
				return errors.New("telemetry sample label is invalid")
			}
		}
	}
	return nil
}
