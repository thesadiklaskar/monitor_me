//go:build !windows

package collector

import (
	"context"
	"errors"
)

func (c *ProcCollector) collectWindows(context.Context) ([]Sample, error) {
	return nil, errors.New("Windows collector is not available in this build")
}
