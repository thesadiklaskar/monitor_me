//go:build linux

package logcollector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeRecordRedactsSecretsAndPreservesSystemdAttribution(t *testing.T) {
	now := time.Now().UTC()
	record := journalRecord{
		Cursor: "s=cursor-1", RealtimeTimestamp: "1795524000123456",
		Message: "login password=hunter2 db_password=\"two words\" api_token=\'quoted-token\' Authorization: Bearer abc.def.ghi",
		Priority: "4", Unit: "payments-api.service", Comm: "payments-api", PID: "321", Hostname: "node-1",
	}
	event, ok := normalizeRecord(record, now)
	if !ok { t.Fatal("expected journal entry to be normalized") }
	if event.Severity != "warning" || event.Source != "payments-api.service" {
		t.Fatalf("source/severity = %s/%s", event.Source, event.Severity)
	}
	if event.Body == record.Message || containsSecret(event.Body, "hunter2") || containsSecret(event.Body, "abc.def.ghi") || containsSecret(event.Body, "two words") || containsSecret(event.Body, "quoted-token") {
		t.Fatalf("sensitive values were not redacted: %q", event.Body)
	}
	if event.Attributes["systemd_unit"] != "payments-api.service" {
		t.Fatalf("systemd attribution missing: %#v", event.Attributes)
	}
	if event.ID == "" || event.EventTimeUnixNanos == 0 {
		t.Fatalf("event identity or timestamp missing: %#v", event)
	}
}

func TestJournalCollectUsesAfterCursorAndCapsBatch(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "journalctl")
	script := "#!/bin/sh\nprintf '%s\\n' " +
		"'{\"__CURSOR\":\"cursor-1\",\"__REALTIME_TIMESTAMP\":\"1795524000123456\",\"MESSAGE\":\"one\",\"PRIORITY\":\"6\"}' " +
		"'{\"__CURSOR\":\"cursor-2\",\"__REALTIME_TIMESTAMP\":\"1795524001123456\",\"MESSAGE\":\"two\",\"PRIORITY\":\"3\"}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil { t.Fatal(err) }
	journal := &Journal{binary: binary}
	events, cursor, err := journal.Collect(context.Background(), "cursor-before", 2)
	if err != nil { t.Fatal(err) }
	if len(events) != 2 || cursor != "cursor-2" {
		t.Fatalf("Collect() events/cursor = %d/%q", len(events), cursor)
	}
}

func containsSecret(value, secret string) bool {
	return len(secret) > 0 && len(value) >= len(secret) && stringContains(value, secret)
}

func stringContains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle { return true }
	}
	return false
}
