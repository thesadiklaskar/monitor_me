package logspool

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testEvent() Event {
	now := time.Now().UTC().UnixNano()
	return Event{
		ID: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		EventTimeUnixNanos: now, ObservedAtUnixNanos: now,
		Source: "sshd.service", Severity: "warning", Body: "invalid login",
		Attributes: map[string]string{"systemd_unit": "sshd.service"},
	}
}

func TestAppendPeekAckPersistsCursor(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "spool"), 2<<20)
	if err != nil { t.Fatal(err) }
	batch, err := NewBatch([]Event{testEvent()}, "s=cursor-1", time.Now().UTC().UnixNano())
	if err != nil { t.Fatal(err) }
	if err := store.Append(batch); err != nil { t.Fatal(err) }
	pending, err := store.Peek(1)
	if err != nil || len(pending) != 1 || pending[0].ID != batch.ID {
		t.Fatalf("Peek() = %#v, %v", pending, err)
	}
	if cursor, err := store.Cursor(); err != nil || cursor != "" {
		t.Fatalf("cursor advanced before server acknowledgement: %q, %v", cursor, err)
	}
	if err := store.Ack(batch.ID); err != nil { t.Fatal(err) }
	if cursor, err := store.Cursor(); err != nil || cursor != "s=cursor-1" {
		t.Fatalf("Cursor() = %q, %v", cursor, err)
	}
	if pending, err := store.Peek(1); err != nil || len(pending) != 0 {
		t.Fatalf("queue was not compacted after acknowledgement: %#v, %v", pending, err)
	}
}

func TestAckMustFollowQueueOrder(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "spool"), 2<<20)
	if err != nil { t.Fatal(err) }
	first, _ := NewBatch([]Event{testEvent()}, "cursor-1", time.Now().UnixNano())
	second, _ := NewBatch([]Event{testEvent()}, "cursor-2", time.Now().UnixNano())
	if err := store.Append(first); err != nil { t.Fatal(err) }
	if err := store.Append(second); err != nil { t.Fatal(err) }
	if err := store.Ack(second.ID); err == nil {
		t.Fatal("out-of-order log batch acknowledgement must fail")
	}
	if cursor, _ := store.Cursor(); cursor != "" {
		t.Fatalf("out-of-order acknowledgement advanced cursor to %q", cursor)
	}
}

func TestRecoverTruncatedTailKeepsCompleteRecords(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "spool")
	store, err := Open(directory, 2<<20)
	if err != nil { t.Fatal(err) }
	batch, err := NewBatch([]Event{testEvent()}, "cursor-1", time.Now().UnixNano())
	if err != nil { t.Fatal(err) }
	if err := store.Append(batch); err != nil { t.Fatal(err) }
	file, err := os.OpenFile(store.path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil { t.Fatal(err) }
	if _, err := file.WriteString("{broken"); err != nil { t.Fatal(err) }
	_ = file.Close()
	reopened, err := Open(directory, 2<<20)
	if err != nil { t.Fatal(err) }
	pending, err := reopened.Peek(4)
	if err != nil || len(pending) != 1 || pending[0].ID != batch.ID {
		t.Fatalf("recovered queue = %#v, %v", pending, err)
	}
}
