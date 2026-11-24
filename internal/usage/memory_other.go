//go:build !darwin && !linux

package usage

type processCPUTime struct{ userMS, systemMS int64 }

func cpuTimes() (processCPUTime, bool) { return processCPUTime{}, false }

func peakRSSBytes() *int64 {
	return nil
}
