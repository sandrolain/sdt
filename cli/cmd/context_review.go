package cmd

import (
	"strings"
)

// ctxReviewBlockHeader is the task-file section that records the verify-step
// verdict of a completed phase. It is checked by lint (advisory) and written by
// `sdt context task review`.
const ctxReviewBlockHeader = "## Review"

// ctxFindingsHeader is the `### Findings` subsection of `## Review` that holds
// one item per review finding, each ending in its verdict token.
const ctxFindingsHeader = "### Findings"

// ctxReviewVerb is the `context status` / proposal-status label for the review
// state, reused so the literal is not repeated across commands.
const ctxReviewVerb = "review"

// ctxReviewVerdicts is the closed vocabulary of the verify-step protocol
// (decision 0013). It is the single source: the help text and the CLI validation
// both derive from it, so a new verdict is added in exactly one place.
var ctxReviewVerdicts = []string{"CONFIRMED", "DISPROVED", "UNVERIFIED"}

// ctxReviewVerdictHelp documents the protocol in the CLI help and templates; it
// is the single source for the three closed verdicts (CONFIRMED, DISPROVED,
// UNVERIFIED) used by the verify-step review.
var ctxReviewVerdictHelp = strings.Join(ctxReviewVerdicts, " | ")

// validReviewVerdict reports whether v is in the closed verdict vocabulary.
func validReviewVerdict(v string) bool {
	for _, want := range ctxReviewVerdicts {
		if v == want {
			return true
		}
	}
	return false
}

// reviewFinding is one finding recorded against a phase: the claim under review
// and the verdict reached on it.
type reviewFinding struct {
	Claim   string
	Verdict string
}

// renderFinding is the checklist body of one finding, without its CLI anchor:
// the claim followed by the verdict as its trailing token.
func renderFinding(f reviewFinding) string {
	return f.Claim + " — " + f.Verdict
}

// hasVerdictToken reports whether an item body ends in one of the closed
// verdicts. The verdict is the trailing token by construction, so a claim that
// merely mentions a verdict word mid-sentence does not count.
func hasVerdictToken(body string) bool {
	fields := strings.Fields(strings.TrimSpace(body))
	if len(fields) == 0 {
		return false
	}
	return validReviewVerdict(fields[len(fields)-1])
}

// appendFindingItems records one checklist item per finding under the
// `### Findings` subsection of the task file's `## Review` block, creating the
// subsection when absent. Item ids are CLI-assigned by the same stamping pass as
// every other checklist.
//
// A finding whose claim and verdict are already present is not repeated, so
// identity is stable across a re-run of the same review. A claim re-stated with
// a different verdict is a new finding and receives a distinct id, so a verdict
// that changed is never silently overwritten.
func appendFindingItems(content string, findings []reviewFinding) string {
	if len(findings) == 0 {
		return content
	}
	content = appendReviewBlock(content, "")
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	review := indexOfTrimmedLine(lines, ctxReviewBlockHeader, 0, len(lines))
	if review < 0 {
		return content
	}
	reviewEnd := sectionEnd(lines, review, 2)
	items := pendingFindingItems(lines, review+1, reviewEnd, findings)
	if len(items) == 0 {
		return content
	}
	if at := indexOfTrimmedLine(lines, ctxFindingsHeader, review+1, reviewEnd); at >= 0 {
		end := sectionAppendIndex(lines, sectionEnd(lines, at, 3))
		lines = insertAt(lines, end, items...)
	} else {
		// A fresh subsection reads before the gate evidence when there is one.
		at := reviewEnd
		for i := review + 1; i < reviewEnd; i++ {
			if headingLevel(lines[i]) > 0 {
				at = i
				break
			}
		}
		block := append([]string{"", ctxFindingsHeader, ""}, items...)
		lines = insertAt(lines, sectionAppendIndex(lines, at), block...)
	}
	stamped, _ := stampChecklistIDs(strings.Join(lines, "\n") + "\n")
	return stamped
}

// pendingFindingItems renders the findings not already recorded in lines[from:to],
// comparing on the anchor-stripped item text so a stamped id never prevents
// deduplication.
func pendingFindingItems(lines []string, from, to int, findings []reviewFinding) []string {
	seen := map[string]bool{}
	if to > len(lines) {
		to = len(lines)
	}
	for i := from; i < to; i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "- [") {
			continue
		}
		// Checklist text first (it drops the `- [ ]` prefix), then the anchor.
		body, _ := splitChecklistAnchor(checklistLineText(lines[i]))
		seen[body] = true
	}
	var items []string
	for _, f := range findings {
		text := renderFinding(f)
		if seen[text] {
			continue
		}
		seen[text] = true
		items = append(items, "- [ ] "+text)
	}
	return items
}

// hasReviewBlock reports whether the task content already carries a Review
// section (heading or `## Review`).
func hasReviewBlock(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == ctxReviewBlockHeader {
			return true
		}
	}
	return false
}

// appendReviewBlock appends the verify-step review section to a task file,
// idempotently (a second call is a no-op).
func appendReviewBlock(content, body string) string {
	if hasReviewBlock(content) {
		return content
	}
	content = strings.TrimRight(content, "\n") + "\n\n"
	content += ctxReviewBlockHeader + "\n\n"
	content += reviewProtocolReminder() + "\n"
	if b := strings.TrimSpace(body); b != "" {
		content += "\n" + b + "\n"
	}
	return content
}

// appendReviewFindings records the caller's findings: it creates the `## Review`
// block when absent, and adds the findings to the block that already exists,
// immediately before the first `###` subsection (the gate record) or at the end
// of the block when it has none.
//
// The block can already exist when `sdt agent gate --record` wrote it first —
// which is why `sdt context task review` uses this instead of appendReviewBlock,
// whose no-op-on-existing contract would silently drop the findings. Findings
// already present are not repeated, so replaying the same `--input` is a no-op.
func appendReviewFindings(content, body string) string {
	body = strings.TrimSpace(body)
	if body == "" || strings.Contains(content, body) {
		return content
	}
	if !hasReviewBlock(content) {
		return appendReviewBlock(content, body)
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	review := indexOfTrimmedLine(lines, ctxReviewBlockHeader, 0, len(lines))
	if review < 0 {
		return content
	}
	end := sectionEnd(lines, review, 2)
	at := end
	for i := review + 1; i < end; i++ {
		if headingLevel(lines[i]) > 0 {
			at = i
			break
		}
	}
	block := append(strings.Split(body, "\n"), "")
	return strings.Join(insertAt(lines, sectionAppendIndex(lines, at), block...), "\n")
}

// reviewProtocolReminder is the fixed protocol text stored in every Review
// block: closed verdicts, evidence, and no self-approval.
func reviewProtocolReminder() string {
	return "Verify-step protocol: each finding ends as **CONFIRMED**, **DISPROVED** or\n" +
		"**UNVERIFIED**, with evidence (command output, file path). An independent\n" +
		"pass validates findings; the author of the phase does not self-approve."
}
