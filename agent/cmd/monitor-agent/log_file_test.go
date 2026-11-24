package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingLogWriterKeepsBoundedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	writerInterface, closeWriter, err := openAgentLogWriter(path)
	if err != nil {
		t.Fatalf("openAgentLogWriter() error = %v", err)
	}
	writer := writerInterface.(*rotatingLogWriter)
	payload := make([]byte, agentLogMaxBytes/2+100)
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := closeWriter(); err != nil {
		t.Fatal(err)
	}

	total := int64(0)
	for _, name := range []string{path, path + ".1", path + ".2", path + ".3"} {
		info, statErr := os.Stat(name)
		if statErr == nil {
			total += info.Size()
		} else if !os.IsNotExist(statErr) {
			t.Fatal(statErr)
		}
	}
	if total > agentLogMaxBytes*(agentLogBackups+1) {
		t.Fatalf("rotated logs exceed bounded size: %d", total)
	}
}
