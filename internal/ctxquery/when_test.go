package ctxquery

import (
	"testing"
	"time"
)

func TestParseWindow(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"10m", 10 * time.Minute, true},
		{"6h", 6 * time.Hour, true},
		{"7d", 7 * 24 * time.Hour, true},
		{"3w", 3 * 7 * 24 * time.Hour, true},
		{"", 0, false},
		{"d", 0, false},
		{"0d", 0, false},
		{"-1d", 0, false},
		{"7s", 0, false},
		{"1mo", 0, false},
		{"1.5d", 0, false},
		{"1d2h", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseWindow(tt.in)
		if tt.ok && (err != nil || got != tt.want) {
			t.Errorf("ParseWindow(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseWindow(%q) = %v; want error", tt.in, got)
		}
	}
}

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"now", now, true},
		{"now-7d", time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), true},
		{"now-6h", time.Date(2026, 9, 28, 6, 0, 0, 0, time.UTC), true},
		{"now-3w", time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC), true},
		{"2026-09-01", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), true},
		{"2026-09-01T10:00:00Z", time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), true},
		{"2026-09-01T10:00:00+02:00", time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC), true},
		{"", time.Time{}, false},
		{"yesterday", time.Time{}, false},
		{"now-2mo", time.Time{}, false},
	}
	for _, tt := range tests {
		got, err := ParseWhen(tt.in, now)
		if tt.ok && (err != nil || !got.Equal(tt.want)) {
			t.Errorf("ParseWhen(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseWhen(%q) = %v; want error", tt.in, got)
		}
	}
}
