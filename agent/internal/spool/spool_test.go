package spool

import (
	"errors"
	"os"
	"path/filepath"
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
