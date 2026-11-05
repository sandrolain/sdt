// Package usage is the counted-usage log: one local, content-free line per
// invocation, so `sdt gain` reports what SDT actually did instead of what it was
// expected to do. A record carries the command path and four numbers — never an
// argument, a path, a query, an environment value or any content.
package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// EnvOff disables the log for every command of the process.
	EnvOff = "SDT_USAGE"
	// EnvSource marks a call SDT makes on its own behalf. Absent means the user
	// ran the command.
	EnvSource = "SDT_USAGE_SOURCE"
	// SourceInternal marks an internal call; SourceCLI is the default.
	SourceInternal = "internal"
	SourceCLI      = "cli"

	// stateDir is the project's derived-state home, shared with the search cache
	// and the semantic snapshot, and gitignored.
	stateDir       = ".sdt"
	usageDir       = "usage"
	filePrefix     = "usage-"
	fileSuffix     = ".jsonl"
	monthLayout    = "2006-01"
	tsLayout       = time.RFC3339
	maxLine        = 4096
	keepMonths     = 6
	dirPermission  = 0o700
	filePermission = 0o600
)

// Entry is one invocation. Hits is a pointer because a command that reports
// "zero results" and one that reports nothing at all are different facts, and
// the hit rate must keep the difference.
type Entry struct {
	TS     string `json:"ts"`
	Cmd    string `json:"cmd"`
	Exit   int    `json:"exit"`
	MS     int64  `json:"ms"`
	Out    int64  `json:"out"`
	Hits   *int   `json:"hits,omitempty"`
	Source string `json:"source,omitempty"`
}

// Dir is the usage directory under a project root.
func Dir(root string) string {
	return filepath.Join(root, stateDir, usageDir)
}

// Append adds one entry to the monthly file its timestamp names, then prunes
// month files the retention window no longer covers. The write is a single
// O_APPEND write of a line under maxLine, so two concurrent invocations cannot
// interleave a record.
func Append(root string, e Entry) error {
	if root == "" {
		return errors.New("usage: no project root")
	}
	ts, err := time.Parse(tsLayout, e.TS)
	if err != nil {
		return fmt.Errorf("usage: bad timestamp %q: %w", e.TS, err)
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(line) > maxLine {
		return fmt.Errorf("usage: record is %d bytes, over the %d-byte line bound", len(line), maxLine)
	}

	dir := Dir(root)
	if err := os.MkdirAll(dir, dirPermission); err != nil {
		return err
	}
	path := filepath.Join(dir, filePrefix+ts.UTC().Format(monthLayout)+fileSuffix)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePermission) // #nosec G304 -- fixed name under the resolved project root
	if err != nil {
		return err
	}
	payload := append(line, '\n')
	if _, werr := f.Write(payload); werr != nil {
		if cerr := f.Close(); cerr != nil {
			return errors.Join(werr, cerr)
		}
		return werr
	}
	if cerr := f.Close(); cerr != nil {
		return cerr
	}
	return prune(dir, ts)
}

// Read returns the recorded entries at or after since, oldest first. A zero
// since reads everything. A malformed line is skipped rather than failing the
// report: a half-written tail must not make the whole history unreadable.
func Read(root string, since time.Time) ([]Entry, error) {
	dir := Dir(root)
	names, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	for _, name := range names {
		if !isMonthFile(name.Name()) {
			continue
		}
		fileEntries, ferr := readFile(filepath.Join(dir, name.Name()), since)
		if ferr != nil {
			return nil, ferr
		}
		entries = append(entries, fileEntries...)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].TS < entries[j].TS })
	return entries, nil
}

// readFile returns the entries of one month file, filtered by since.
func readFile(path string, since time.Time) ([]Entry, error) {
	f, err := os.Open(path) // #nosec G304 -- a month file of the resolved usage directory
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024), maxLine)
	for scanner.Scan() {
		var e Entry
		if uerr := json.Unmarshal(scanner.Bytes(), &e); uerr != nil {
			continue
		}
		if !since.IsZero() {
			ts, terr := time.Parse(tsLayout, e.TS)
			if terr != nil || ts.Before(since) {
				continue
			}
		}
		entries = append(entries, e)
	}
	if serr := scanner.Err(); serr != nil {
		return nil, serr
	}
	return entries, nil
}

// prune removes month files older than the retention window, measured from now.
func prune(dir string, now time.Time) error {
	cutoff := now.UTC().AddDate(0, -keepMonths, 0).Format(monthLayout)
	names, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		if !isMonthFile(name.Name()) {
			continue
		}
		month := strings.TrimSuffix(strings.TrimPrefix(name.Name(), filePrefix), fileSuffix)
		if month < cutoff {
			if rerr := os.Remove(filepath.Join(dir, name.Name())); rerr != nil {
				return rerr
			}
		}
	}
	return nil
}

// isMonthFile reports whether name is one of this package's month files.
func isMonthFile(name string) bool {
	if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
		return false
	}
	month := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
	_, err := time.Parse(monthLayout, month)
	return err == nil
}

// ParseWindow turns a --since value into a lower bound: "all" has no bound, a
// day count ("7d") is that many hours, and any Go duration ("24h", "90m") is
// taken as written.
func ParseWindow(v string, now time.Time) (time.Time, error) {
	v = strings.TrimSpace(v)
	switch v {
	case "", "all":
		return time.Time{}, nil
	case "0":
		return time.Time{}, nil
	}
	if days, ok := strings.CutSuffix(v, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q: use a duration (24h), days (7d) or all", v)
		}
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: use a duration (24h), days (7d) or all", v)
	}
	return now.Add(-d), nil
}

// The pending record of this process. A CLI runs one command at a time, so
// package state is the honest shape — the same reason logging keeps a default
// logger.
var (
	pending Entry
	began   time.Time
	started bool
)

// Begin opens the record for the command about to run.
func Begin(cmd string) {
	now := time.Now().UTC()
	began = time.Now()
	pending = Entry{TS: now.Format(tsLayout), Cmd: cmd}
	if strings.EqualFold(os.Getenv(EnvSource), SourceInternal) {
		pending.Source = SourceInternal
	}
	started = true
}

// Hits records how many items the command returned. Only a command that knows
// the number reports it; leaving it unset keeps the hit rate honest.
func Hits(n int) {
	if !started {
		return
	}
	pending.Hits = &n
}

// CountOut adds bytes written to stdout.
func CountOut(n int64) {
	if !started {
		return
	}
	pending.Out += n
}

// Finish closes the record with its exit code and duration and appends it under
// root. It is idempotent, and a root of "" (no project, or the log disabled)
// drops the record without an error.
func Finish(root string, code int) error {
	if !started {
		return nil
	}
	started = false
	pending.Exit = code
	pending.MS = time.Since(began).Milliseconds()
	e := pending
	pending = Entry{}
	if root == "" {
		return nil
	}
	return Append(root, e)
}
