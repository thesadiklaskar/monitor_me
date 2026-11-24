//go:build linux

package logcollector

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/monitor-me/agent/internal/logspool"
)

const maxJournalLine = 512 << 10

var (
	assignmentSecret = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization)\b(\s*[:=]\s*)([^\s,;]+)`)
	bearerSecret = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
)

type Journal struct {
	binary string
}

type journalRecord struct {
	Cursor string `json:"__CURSOR"`
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
	Message string `json:"MESSAGE"`
	Priority string `json:"PRIORITY"`
	Unit string `json:"_SYSTEMD_UNIT"`
	Identifier string `json:"SYSLOG_IDENTIFIER"`
	Comm string `json:"_COMM"`
	PID string `json:"_PID"`
	Hostname string `json:"_HOSTNAME"`
}

func NewJournal() *Journal {
	return &Journal{binary: "journalctl"}
}

func (j *Journal) Available() bool {
	_, err := exec.LookPath(j.binary)
	return err == nil
}

// Collect reads at most limit records after the last acknowledged journal cursor.
// It never writes the cursor file itself: the durable spool advances the cursor
// only after the control plane acknowledges the corresponding log batch.
func (j *Journal) Collect(ctx context.Context, cursor string, limit int) ([]logspool.Event, string, error) {
	if limit < 1 || limit > 32 {
		return nil, cursor, errors.New("journal batch limit must be between 1 and 32")
	}
	if _, err := exec.LookPath(j.binary); err != nil {
		return nil, cursor, fmt.Errorf("journalctl is not available: %w", err)
	}
	events, nextCursor, err := j.collect(ctx, cursor, limit)
	if err != nil && cursor != "" && ctx.Err() == nil {
		// Journals rotate. If the saved cursor has expired, resume from a small
		// recent window; event IDs remain stable so overlap is deduplicated.
		return j.collect(ctx, "", limit)
	}
	return events, nextCursor, err
}

func (j *Journal) collect(ctx context.Context, cursor string, limit int) ([]logspool.Event, string, error) {
	args := []string{"--no-pager", "--output=json", "--all", "--quiet"}
	if cursor != "" {
		args = append(args, "--after-cursor="+cursor)
	} else {
		args = append(args, "--since=-5m", "--lines=128")
	}
	command := exec.CommandContext(ctx, j.binary, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, cursor, fmt.Errorf("open journalctl output: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return nil, cursor, fmt.Errorf("start journalctl: %w", err)
	}

	events := make([]logspool.Event, 0, limit)
	nextCursor := cursor
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), maxJournalLine)
	reachedLimit := false
	for scanner.Scan() {
		var record journalRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, cursor, fmt.Errorf("decode journald JSON event: %w", err)
		}
		event, ok := normalizeRecord(record, time.Now().UTC())
		if !ok {
			continue
		}
		events = append(events, event)
		nextCursor = record.Cursor
		if len(events) >= limit {
			reachedLimit = true
			_ = command.Process.Kill()
			break
		}
	}
	scanErr := scanner.Err()
	if scanErr != nil {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if scanErr != nil {
		return nil, cursor, fmt.Errorf("read journald output: %w", scanErr)
	}
	if waitErr != nil && !reachedLimit && ctx.Err() == nil {
		message := strings.TrimSpace(stderr.String())
		if len(message) > 512 {
			message = message[:512]
		}
		if message == "" {
			return nil, cursor, fmt.Errorf("journalctl exited unsuccessfully: %w", waitErr)
		}
		return nil, cursor, fmt.Errorf("journalctl exited unsuccessfully: %s", message)
	}
	if ctx.Err() != nil {
		return nil, cursor, ctx.Err()
	}
	return events, nextCursor, nil
}

func normalizeRecord(record journalRecord, observedAt time.Time) (logspool.Event, bool) {
	if record.Cursor == "" || record.Message == "" {
		return logspool.Event{}, false
	}
	eventTime := observedAt
	if record.RealtimeTimestamp != "" {
		microseconds, err := strconv.ParseInt(record.RealtimeTimestamp, 10, 64)
		if err == nil && microseconds > 0 {
			eventTime = time.UnixMicro(microseconds).UTC()
		}
	}
	cursorHash := sha256.Sum256([]byte(record.Cursor))
	source := record.Unit
	if source == "" {
		source = record.Identifier
	}
	if source == "" {
		source = record.Comm
	}
	if source == "" {
		source = "journald"
	}
	if len(source) > 128 {
		source = source[:128]
	}
	body := strings.ReplaceAll(record.Message, "\x00", "")
	body = bearerSecret.ReplaceAllString(body, "Bearer [REDACTED]")
	body = assignmentSecret.ReplaceAllString(body, "$1$2[REDACTED]")
	if len(body) > 16<<10 {
		body = body[:16<<10]
	}

	attributes := make(map[string]string, 4)
	if record.Unit != "" {
		attributes["systemd_unit"] = bounded(record.Unit, 256)
	}
	if record.Comm != "" {
		attributes["comm"] = bounded(record.Comm, 128)
	}
	if record.PID != "" {
		attributes["pid"] = bounded(record.PID, 32)
	}
	if record.Hostname != "" {
		attributes["hostname"] = bounded(record.Hostname, 253)
	}
	return logspool.Event{
		ID: hex.EncodeToString(cursorHash[:]),
		EventTimeUnixNanos: eventTime.UnixNano(),
		ObservedAtUnixNanos: observedAt.UnixNano(),
		Source: source,
		Severity: severity(record.Priority),
		Body: body,
		Attributes: attributes,
	}, true
}

func severity(priority string) string {
	switch priority {
	case "0":
		return "emergency"
	case "1":
		return "alert"
	case "2":
		return "critical"
	case "3":
		return "error"
	case "4":
		return "warning"
	case "5":
		return "notice"
	case "6":
		return "info"
	case "7":
		return "debug"
	default:
		return "unknown"
	}
}

func bounded(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return -1
		}
		return r
	}, value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
