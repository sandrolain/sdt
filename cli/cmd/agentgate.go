package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// `sdt agent gate --record` writes the ladder's outcome into the task file's
// `## Review` block, so a green run is evidence the next session can read back
// instead of a claim in a chat log. The ladder itself does not change: without
// `--record` the command writes nothing at all.

// ctxGateRecordHeader is the `### Gate` subsection appended under `## Review`.
const ctxGateRecordHeader = "### Gate"

// ctxGateRecordSince is the day the gate-record feature shipped.
// `lintGateEvidence` skips a completed task file last updated before this day
// when its plan is no longer active, so the one-off absence of a record on
// historical phases is not reported as a corpus-wide flood.
var ctxGateRecordSince = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

// gateStepResult is one ladder rung as the record stores it: the step name, the
// status it reached and how long it took.
type gateStepResult struct {
	Step     string
	Status   string
	Duration time.Duration
}

// writeGateRecord appends one gate run to the task file at path. The file must
// already exist: the gate records evidence about work the agent did, so it never
// creates the task file it reports on.
func writeGateRecord(path string, stamp time.Time, failed string, results []gateStepResult) error {
	//#nosec G304 -- path resolved from the corpus, not from user input
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no task file at %s; create it with `sdt context task add` before recording the gate", path)
		}
		return err
	}
	updated := appendGateRecord(string(data), stamp, failed, results)
	//#nosec G304 G306 G703 -- path resolved from the corpus, not from user input
	return os.WriteFile(path, []byte(updated), 0o644)
}

// appendGateRecord writes one gate run under the `### Gate` subsection of the
// task file's `## Review` block, creating either when absent. Runs accumulate
// newest-last: nothing an earlier run wrote is ever rewritten. The run is headed
// by its timestamp and, when it did not pass, by the step it failed at — a
// failing run is recorded with the status it actually reached, never as a pass.
func appendGateRecord(content string, stamp time.Time, failed string, results []gateStepResult) string {
	content = appendReviewBlock(content, "")
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	review := indexOfTrimmedLine(lines, ctxReviewBlockHeader, 0, len(lines))
	if review < 0 {
		// appendReviewBlock guarantees the section; keep the function total.
		return content
	}
	run := gateRunLines(stamp, failed, results)
	reviewEnd := sectionEnd(lines, review, 2)
	if gate := indexOfTrimmedLine(lines, ctxGateRecordHeader, review+1, reviewEnd); gate >= 0 {
		end := sectionAppendIndex(lines, sectionEnd(lines, gate, 3))
		return strings.Join(insertAt(lines, end, append([]string{""}, run...)...), "\n") + "\n"
	}
	block := append([]string{"", ctxGateRecordHeader, ""}, run...)
	return strings.Join(insertAt(lines, sectionAppendIndex(lines, reviewEnd), block...), "\n") + "\n"
}

// gateRunLines renders one run: a heading carrying the timestamp and, when the
// run did not pass, the failure, then one line per step with its status and
// duration.
func gateRunLines(stamp time.Time, failed string, results []gateStepResult) []string {
	head := stamp.UTC().Format(time.RFC3339) + " (sdt agent gate)"
	if failed != "" {
		head += " — FAILED at step " + failed
	} else {
		head += " — passed"
	}
	out := make([]string, 0, len(results)+1)
	out = append(out, head)
	for _, r := range results {
		out = append(out, fmt.Sprintf("- %s: %s (%.2fs)", r.Step, r.Status, r.Duration.Seconds()))
	}
	return out
}

// indexOfTrimmedLine returns the first index in [from, to) whose trimmed line
// equals want, or -1.
func indexOfTrimmedLine(lines []string, want string, from, to int) int {
	if to > len(lines) {
		to = len(lines)
	}
	for i := from; i < to; i++ {
		if strings.TrimSpace(lines[i]) == want {
			return i
		}
	}
	return -1
}

// headingLevel is the ATX heading level of a line (1-6), or 0 when it is not a
// heading.
func headingLevel(line string) int {
	t := strings.TrimSpace(line)
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(t) && t[n] != ' ') {
		return 0
	}
	return n
}

// sectionEnd returns the exclusive end of the section starting at start: the
// index of the heading that closes it, or len(lines) when it runs to the end of
// the document. It is the bound to pass as `to` to a half-open scan.
func sectionEnd(lines []string, start, maxLevel int) int {
	for i := start + 1; i < len(lines); i++ {
		if lvl := headingLevel(lines[i]); lvl > 0 && lvl <= maxLevel {
			return i
		}
	}
	return len(lines)
}

// sectionAppendIndex converts a section's exclusive end into the index at which
// new lines land *after* its content: past the last line when the section ends
// the document, and after trimming the blank lines that precede the heading
// which closes it otherwise.
func sectionAppendIndex(lines []string, end int) int {
	if end >= len(lines) {
		return len(lines)
	}
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

// insertAt returns lines with block inserted at index i.
func insertAt(lines []string, i int, block ...string) []string {
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:i]...)
	out = append(out, block...)
	out = append(out, lines[i:]...)
	return out
}
