package ctxquery

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseWhen resolves a date expression against an injected now. Accepted forms:
// YYYY-MM-DD (UTC midnight), RFC3339, "now" and "now-<N><unit>" with units
// m/h/d/w. The result is normalized to UTC.
func ParseWhen(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if s == "now" {
		return now.UTC(), nil
	}
	if off, ok := strings.CutPrefix(s, "now-"); ok {
		d, err := ParseWindow(off)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid relative date %q: %w", s, err)
		}
		return now.Add(-d).UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD, RFC3339, now or now-<N><unit>", s)
}

// ParseWindow parses a window such as 10m, 6h, 7d or 3w. Only minutes, hours,
// days and weeks are supported; months and years are intentionally rejected.
func ParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty window")
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid window %q: want <N><unit>", s)
	}
	var d time.Duration
	switch s[len(s)-1] {
	case 'm':
		d = time.Minute
	case 'h':
		d = time.Hour
	case 'd':
		d = 24 * time.Hour
	case 'w':
		d = 7 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("invalid unit %q in %q: want m, h, d or w", string(s[len(s)-1]), s)
	}
	return time.Duration(n) * d, nil
}

// ParseTimestamp parses a stored frontmatter timestamp best-effort (zero on
// failure); it never interprets relative expressions.
func ParseTimestamp(s string) time.Time { return parseTimestamp(s) }

// parseTimestamp parses a stored frontmatter timestamp best-effort (zero on
// failure); it never interprets relative expressions.
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
