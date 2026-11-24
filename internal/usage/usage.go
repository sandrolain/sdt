// Package usage writes one local counted-usage line per invocation. Argument
// capture is opt-in and bounded; aggregate reports never expose captured argv.
package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func allocatedBytes() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.TotalAlloc
}

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
	maxArgsBytes   = 1024
	dirPermission  = 0o700
	filePermission = 0o600
)

// Entry is one invocation. Hits is a pointer because a command that reports
// "zero results" and one that reports nothing at all are different facts, and
// the hit rate must keep the difference.
type Entry struct {
	TS               string   `json:"ts"`
	Cmd              string   `json:"cmd"`
	Exit             int      `json:"exit"`
	MS               int64    `json:"ms"`
	Out              int64    `json:"out"`
	Hits             *int     `json:"hits,omitempty"`
	Source           string   `json:"source,omitempty"`
	RSSBytes         *int64   `json:"rss_bytes,omitempty"`
	Argv             []string `json:"argv,omitempty"`
	ArgvTruncated    bool     `json:"argv_truncated,omitempty"`
	GoAllocatedBytes *int64   `json:"go_allocated_bytes,omitempty"`
	CPUUserMS        *int64   `json:"cpu_user_ms,omitempty"`
	CPUSystemMS      *int64   `json:"cpu_system_ms,omitempty"`
	CPUPeakPercent   *float64 `json:"cpu_peak_percent,omitempty"`
}

// BeginOptions controls which optional data is collected for one invocation.
// Memory defaults to true when Begin is called without explicit options, for
// compatibility with the existing RSS record.
type BeginOptions struct {
	CaptureArgs bool
	Args        []string
	Memory      bool
	CPU         bool
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
	pending      Entry
	began        time.Time
	started      bool
	beginOptions BeginOptions
	allocStart   uint64
	cpuStop      chan struct{}
	cpuDone      chan struct{}
	cpuPeak      float64
	cpuSamples   int
	cpuStart     processCPUTime
)

// Begin opens the record for the command about to run.
func Begin(cmd string, options ...BeginOptions) {
	now := time.Now().UTC()
	began = time.Now()
	pending = Entry{TS: now.Format(tsLayout), Cmd: cmd}
	beginOptions = BeginOptions{Memory: true}
	if len(options) > 0 {
		beginOptions = options[0]
	}
	if beginOptions.Memory {
		allocStart = allocatedBytes()
	}
	if beginOptions.CaptureArgs {
		pending.Argv, pending.ArgvTruncated = boundedArgs(beginOptions.Args)
	}
	if strings.EqualFold(os.Getenv(EnvSource), SourceInternal) {
		pending.Source = SourceInternal
	}
	started = true
	if beginOptions.CPU {
		cpuPeak = 0
		cpuSamples = 0
		cpuStart, _ = cpuTimes()
		cpuStop, cpuDone = make(chan struct{}), make(chan struct{})
		go sampleCPU(cpuStop, cpuDone)
	}
}

func sampleCPU(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	lastWall, lastCPU := time.Now(), cpuStart
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			current, ok := cpuTimes()
			if !ok {
				continue
			}
			wallMS := now.Sub(lastWall).Milliseconds()
			cpuMS := current.userMS + current.systemMS - lastCPU.userMS - lastCPU.systemMS
			if wallMS > 0 && cpuMS >= 0 {
				percent := float64(cpuMS) * 100 / float64(wallMS)
				cpuSamples++
				if percent > cpuPeak {
					cpuPeak = percent
				}
			}
			lastWall, lastCPU = now, current
		}
	}
}

func boundedArgs(args []string) ([]string, bool) {
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, true
	}
	if len(encoded) <= maxArgsBytes {
		return append([]string(nil), args...), false
	}

	bounded := make([]string, 0, len(args))
	used := 2 // JSON array brackets.
	for _, arg := range args {
		item, err := json.Marshal(arg)
		if err != nil {
			return bounded, true
		}
		separator := 0
		if len(bounded) > 0 {
			separator = 1
		}
		if used+separator+len(item) <= maxArgsBytes {
			bounded = append(bounded, arg)
			used += separator + len(item)
			continue
		}

		budget := maxArgsBytes - used - separator
		prefix := truncateStringToJSONBudget(arg, budget)
		prefixJSON, err := json.Marshal(prefix)
		if err != nil {
			return bounded, true
		}
		if len(prefixJSON) <= budget && (prefix != "" || arg == "") {
			bounded = append(bounded, prefix)
		}
		return bounded, true
	}
	return bounded, true
}

func truncateStringToJSONBudget(value string, budget int) string {
	if budget < 2 {
		return ""
	}
	var prefix strings.Builder
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		candidate := prefix.String() + string(r)
		encoded, err := json.Marshal(candidate)
		if err != nil || len(encoded) > budget {
			break
		}
		prefix.WriteRune(r)
		value = value[size:]
	}
	return prefix.String()
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
	if beginOptions.Memory {
		pending.RSSBytes = peakRSSBytes()
		end := allocatedBytes()
		if end >= allocStart {
			delta := end - allocStart
			if delta <= uint64(^uint64(0)>>1) {
				value := int64(delta)
				pending.GoAllocatedBytes = &value
			}
		}
	}
	if beginOptions.CPU {
		close(cpuStop)
		<-cpuDone
		if end, ok := cpuTimes(); ok {
			user, system := end.userMS-cpuStart.userMS, end.systemMS-cpuStart.systemMS
			pending.CPUUserMS, pending.CPUSystemMS = &user, &system
			if cpuSamples == 0 && time.Since(began) > 0 {
				peak := float64(user+system) * 100 / float64(time.Since(began).Milliseconds()+1)
				cpuPeak = peak
			}
			peak := cpuPeak
			pending.CPUPeakPercent = &peak
		}
	}
	e := pending
	pending = Entry{}
	beginOptions = BeginOptions{}
	if root == "" {
		return nil
	}
	return Append(root, e)
}
