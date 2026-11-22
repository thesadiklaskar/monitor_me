package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var (
	ErrExpired = errors.New("agent session has expired")
	uuidRE     = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")
)

// Credentials contains the session secret required after one-time enrollment.
// The file is private to the service account and is replaced atomically.
type Credentials struct {
	AgentID                  string `json:"agent_id"`
	ServerID                 string `json:"server_id"`
	SessionToken             string `json:"session_token"`
	ProtocolVersion          string `json:"protocol_version"`
	ExpiresAtUnixMillis      int64  `json:"expires_at_unix_millis"`
	HeartbeatIntervalSeconds int32  `json:"heartbeat_interval_seconds"`
}

func Load(path string) (Credentials, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Credentials{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Credentials{}, errors.New("agent credential state must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Credentials{}, errors.New("agent credential state permissions must not allow group or other access")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("read agent credential state: %w", err)
	}
	var credentials Credentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return Credentials{}, fmt.Errorf("decode agent credential state: %w", err)
	}
	if err := validate(credentials); err != nil {
		return Credentials{}, err
	}
	if credentials.ExpiresAtUnixMillis <= time.Now().UTC().UnixMilli() {
		return Credentials{}, ErrExpired
	}
	return credentials, nil
}

func Save(path string, credentials Credentials) error {
	if err := validate(credentials); err != nil {
		return err
	}
	if credentials.ExpiresAtUnixMillis <= time.Now().UTC().UnixMilli() {
		return ErrExpired
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create agent state directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure agent state directory: %w", err)
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("agent credential state must not be a symbolic link")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect agent credential state: %w", err)
	}

	encoded, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("encode agent credential state: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".agent-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary agent state file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure temporary agent state file: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return fmt.Errorf("write agent credential state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync agent credential state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close agent credential state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace agent credential state: %w", err)
	}
	dir, err := os.Open(directory)
	if err == nil {
		if syncErr := dir.Sync(); syncErr != nil {
			dir.Close()
			return fmt.Errorf("sync agent state directory: %w", syncErr)
		}
		_ = dir.Close()
	}
	return nil
}

func Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove agent credential state: %w", err)
	}
	return nil
}

func validate(credentials Credentials) error {
	if !uuidRE.MatchString(credentials.AgentID) || !uuidRE.MatchString(credentials.ServerID) ||
		len(credentials.SessionToken) < 32 || len(credentials.SessionToken) > 128 ||
		credentials.ProtocolVersion != "monitor.v1" ||
		credentials.ExpiresAtUnixMillis <= 0 ||
		credentials.HeartbeatIntervalSeconds < 5 || credentials.HeartbeatIntervalSeconds > 300 {
		return errors.New("agent credential state is invalid")
	}
	return nil
}
