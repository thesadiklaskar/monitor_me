//go:build !windows

package spool

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
