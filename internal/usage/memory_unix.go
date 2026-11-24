//go:build darwin || linux

package usage

import (
	"runtime"
	"syscall"
)

type processCPUTime struct{ userMS, systemMS int64 }

func cpuTimes() (processCPUTime, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return processCPUTime{}, false
	}
	return processCPUTime{
		userMS:   usage.Utime.Sec*1000 + int64(usage.Utime.Usec)/1000,
		systemMS: usage.Stime.Sec*1000 + int64(usage.Stime.Usec)/1000,
	}, true
}

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
