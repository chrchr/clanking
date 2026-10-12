// Package transport carries cue names and working directories from hook clients to the daemon over a Unix socket: datagrams where supported, one stream connection per cue on Windows.
package transport

import (
	"errors"
	"io/fs"
	"os"
	"time"
)

const (
	maxDatagram    = 8192
	fieldSeparator = " "
	sendTimeout    = 100 * time.Millisecond
)

func removeStale(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Deliver makes one attempt to reach the daemon; when that fails it spawns one and retries until timeout.
func Deliver(attempt, spawn func() error, timeout, interval time.Duration) error {
	if attempt() == nil {
		return nil
	}
	if err := spawn(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		time.Sleep(interval)
		err := attempt()
		if err == nil || time.Now().After(deadline) {
			return err
		}
	}
}
