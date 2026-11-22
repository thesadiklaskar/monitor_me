package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validCredentials() Credentials {
	return Credentials{
		AgentID:                  "e9e3d6df-8cbe-4d48-92e6-450fb7c132a1",
		ServerID:                 "5c44c3e5-115b-4b81-84b7-07332f18a70f",
		SessionToken:             "01234567890123456789012345678901",
		ProtocolVersion:          "monitor.v1",
		ExpiresAtUnixMillis:      time.Now().Add(time.Hour).UnixMilli(),
		HeartbeatIntervalSeconds: 15,
	}
}

func TestCredentialsSurviveRestartWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "agent-state.json")
	expected := validCredentials()
	if err := Save(path, expected); err != nil {
		t.Fatal(err)
	}
	actual, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual.AgentID != expected.AgentID || actual.ServerID != expected.ServerID ||
		actual.SessionToken != expected.SessionToken {
		t.Fatalf("loaded credentials differ from saved credentials: %#v", actual)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential file mode = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("credential directory mode = %o, want 700", dirInfo.Mode().Perm())
	}
}

func TestLoadRejectsSymlinkAndInsecurePermissions(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("credential symlink must be rejected")
	}
	path := filepath.Join(directory, "insecure.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("credential file readable by other users must be rejected")
	}
}

func TestExpiredCredentialIsNotSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	credentials := validCredentials()
	credentials.ExpiresAtUnixMillis = time.Now().Add(-time.Minute).UnixMilli()
	if err := Save(path, credentials); !errors.Is(err, ErrExpired) {
		t.Fatalf("Save() error = %v, want ErrExpired", err)
	}
}
