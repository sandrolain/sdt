//go:build darwin || linux

package usage

import (
	"runtime"
	"syscall"
)

func peakRSSBytes() *int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil || usage.Maxrss < 0 {
		return nil
	}
	bytes := usage.Maxrss
	if runtime.GOOS == "linux" {
		bytes *= 1024
	}
	return &bytes
}
