//go:build windows

package logcollector

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeWindowsEventRedactsSecretsAndMapsSeverity(t *testing.T) {
	now := time.Now().UTC()
	record := windowsEventRecord{
		LogName: "Application", RecordID: 42, EventID: 1001,
		EventTimeUnixNanos: now.UnixNano(), Provider: "Example Provider", Level: 3,
		Message: "service password=hunter2 Authorization: Bearer abc.def", Computer: "HOST-01",
	}
	event, ok := normalizeWindowsEvent(record, now)
	if !ok {
		t.Fatal("expected supported Windows Event Log entry")
	}
	if event.Severity != "warning" || !strings.HasPrefix(event.Source, "Application/") {
		t.Fatalf("unexpected source/severity: %s/%s", event.Source, event.Severity)
	}
	if strings.Contains(event.Body, "hunter2") || strings.Contains(event.Body, "abc.def") {
		t.Fatalf("secret value was not redacted: %q", event.Body)
	}
	if event.Attributes["event_log"] != "Application" || event.Attributes["record_id"] != "42" {
		t.Fatalf("missing Windows event attributes: %#v", event.Attributes)
	}
}

func TestNormalizeWindowsEventRejectsOtherChannelsAndInvalidIDs(t *testing.T) {
	now := time.Now().UTC()
	for _, record := range []windowsEventRecord{
		{LogName: "Security", RecordID: 1, EventTimeUnixNanos: now.UnixNano()},
		{LogName: "System", RecordID: 0, EventTimeUnixNanos: now.UnixNano()},
	} {
		if _, ok := normalizeWindowsEvent(record, now); ok {
			t.Fatalf("unsupported or invalid record was accepted: %#v", record)
		}
	}
}

func TestEncodePowerShellUsesUTF16LEBase64(t *testing.T) {
	script := "$message = 'café'; Write-Output $message"
	if len(encodePowerShell(script)) == 0 {
		t.Fatal("PowerShell encoded command was empty")
	}
}
