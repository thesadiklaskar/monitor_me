package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	agentLogMaxBytes = 5 * 1024 * 1024
	agentLogBackups  = 3
)

type rotatingLogWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func openAgentLogWriter(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("create agent log directory: %w", err)
	}
	writer := &rotatingLogWriter{path: path}
	if err := writer.open(); err != nil {
		return nil, nil, err
	}
	return writer, writer.Close, nil
}

func (writer *rotatingLogWriter) open() error {
	file, err := os.OpenFile(writer.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open agent log: %w", err)
	}
	writer.file = file
	return nil
}

func (writer *rotatingLogWriter) Write(value []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	info, err := writer.file.Stat()
	if err != nil {
		return 0, fmt.Errorf("inspect agent log: %w", err)
	}
	if info.Size()+int64(len(value)) > agentLogMaxBytes {
		if err := writer.rotate(); err != nil {
			return 0, err
		}
	}
	return writer.file.Write(value)
}

func (writer *rotatingLogWriter) rotate() error {
	if writer.file != nil {
		if err := writer.file.Close(); err != nil {
			return fmt.Errorf("close agent log for rotation: %w", err)
		}
		writer.file = nil
	}

	var rotationErr error
	for index := agentLogBackups; index >= 1; index-- {
		source := writer.path
		if index > 1 {
			source = fmt.Sprintf("%s.%d", writer.path, index-1)
		}
		target := fmt.Sprintf("%s.%d", writer.path, index)
		if _, err := os.Stat(source); err != nil {
			if !os.IsNotExist(err) {
				rotationErr = fmt.Errorf("inspect previous agent log: %w", err)
				break
			}
			continue
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			rotationErr = fmt.Errorf("remove old agent log backup: %w", err)
			break
		}
		if err := os.Rename(source, target); err != nil {
			rotationErr = fmt.Errorf("rotate agent log: %w", err)
			break
		}
	}

	if err := writer.open(); err != nil {
		if rotationErr != nil {
			return fmt.Errorf("%v; reopen agent log: %w", rotationErr, err)
		}
		return err
	}
	return rotationErr
}

func (writer *rotatingLogWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.file == nil {
		return nil
	}
	err := writer.file.Close()
	writer.file = nil
	return err
}
