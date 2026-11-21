//go:build !darwin && !linux

package usage

func peakRSSBytes() *int64 {
	return nil
}
