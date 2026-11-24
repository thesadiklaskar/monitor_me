//go:build !linux && !windows

package logcollector

import (
	"context"
	"errors"
	"time"

	"example.com/monitor-me/agent/internal/logspool"
)

// Journal is an explicit unsupported-platform stub. Windows Event Log and
// macOS unified-log collectors are not advertised until their native readers exist.
type Journal struct{}

func NewJournal() *Journal { return &Journal{} }
func (j *Journal) Available() bool { return false }
func (j *Journal) Capability() string { return "" }
func (j *Journal) PollInterval() time.Duration { return 10 * time.Second }
func (j *Journal) Collect(context.Context, string, int) ([]logspool.Event, string, error) {
	return nil, "", errors.New("native log collection is not supported on this platform yet")
}
