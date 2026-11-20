//go:build unix

package filelock

import (
	"os"
	"syscall"
)

// WithLock serializes callers that open the same lock file.
func WithLock(path string, fn func() error) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //#nosec G304 -- callers choose project-owned lock paths
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // close releases the kernel lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // close releases the lock on failure too
	return fn()
}
