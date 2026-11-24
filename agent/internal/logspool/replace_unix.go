//go:build !windows

package logspool

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
