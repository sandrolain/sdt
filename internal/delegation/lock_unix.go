//go:build unix

package delegation

import (
	"os"
	"syscall"
)

func withFileLock(path string, fn func() error) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //#nosec G304 -- path is under the project-owned delegation state directory
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // lock release is handled by the kernel on close
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // close releases the lock on failure too
	return fn()
}
