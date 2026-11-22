package spool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WorkRCS/monitor_me/agent/internal/collector"
)

func fixtureBatch(t *testing.T, value float64) Batch {
	t.Helper()
	batch, err := NewBatch([]collector.Sample{{
		Name: "host.memory.used", Value: value, Unit: "bytes",
		ObservedAtUnixNanos: time.Now().UnixNano(),
	}}, time.Now().UnixNano())
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestSpoolSurvivesReopenAndAcknowledgesDurably(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	first := fixtureBatch(t, 100)
	second := fixtureBatch(t, 200)
	if err := store.Append(first); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(second); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(directory, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := reopened.Peek(10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("Peek() = %d batches, %v", len(pending), err)
	}
	if err := reopened.Ack([]string{first.ID}); err != nil {
		t.Fatal(err)
	}
	pending, err = reopened.Peek(10)
	if err != nil || len(pending) != 1 || pending[0].ID != second.ID {
		t.Fatalf("unexpected pending batches after ack: %#v, %v", pending, err)
	}
}

func TestSpoolEnforcesQuotaAndPrivateFileMode(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	batch := fixtureBatch(t, 123)
	store.maxBytes = 1
	if err := store.Append(batch); !errors.Is(err, ErrSpoolFull) {
		t.Fatalf("Append() error = %v, want ErrSpoolFull", err)
	}
	info, err := os.Stat(filepath.Join(directory, "queue.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("spool file permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestSpoolRecoveryKeepsCommittedRecordsAndDropsOnlyTruncatedTail(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	first := fixtureBatch(t, 100)
	if err := store.Append(first); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(directory, "queue.jsonl"), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{\"id\":\"partially-written"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, err := Open(directory, 1<<20)
	if err != nil {
		t.Fatalf("Open() should recover an incomplete trailing WAL record: %v", err)
	}
	pending, err := recovered.Peek(10)
	if err != nil || len(pending) != 1 || pending[0].ID != first.ID {
		t.Fatalf("recovery did not preserve the committed batch: %#v, %v", pending, err)
	}
}

func TestSpoolRejectsNonCanonicalBatchIdentifiers(t *testing.T) {
	store, err := Open(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	valid := fixtureBatch(t, 42)
	uppercase := valid
	uppercase.ID = strings.ToUpper(valid.ID)
	if err := store.Append(uppercase); err == nil {
		t.Fatal("uppercase batch IDs must be rejected before they enter the local WAL")
	}
	nonHexadecimal := valid
	nonHexadecimal.ID = "gggggggggggggggggggggggggggggggg"
	if err := store.Append(nonHexadecimal); err == nil {
		t.Fatal("non-hexadecimal batch IDs must be rejected before they enter the local WAL")
	}
	if pending, err := store.Peek(10); err != nil || len(pending) != 0 {
		t.Fatalf("invalid batch IDs must not alter the WAL: pending=%d error=%v", len(pending), err)
	}
}
