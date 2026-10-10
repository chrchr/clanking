package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

const lockMode = 0o600

// Lock takes the single-instance lock, reporting false when another daemon holds it.
func Lock(path string) (*os.File, bool, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("lock file %s is a symlink", path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, lockMode)
	if err != nil {
		return nil, false, err
	}
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	err = windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return nil, false, f.Close()
	}
	if err != nil {
		return nil, false, errors.Join(err, f.Close())
	}
	return f, true, nil
}
