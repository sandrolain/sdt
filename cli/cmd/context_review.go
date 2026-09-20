package cmd

import (
	"strings"
)

// ctxReviewBlockHeader is the task-file section that records the verify-step
// verdict of a completed phase. It is checked by lint (advisory) and written by
// `sdt context task review`.
const ctxReviewBlockHeader = "## Review"

// ctxReviewVerb is the `context status` / proposal-status label for the review
// state, reused so the literal is not repeated across commands.
const ctxReviewVerb = "review"

// ctxReviewVerdictHelp documents the protocol in the CLI help and templates;
// it is the single source for the three closed verdicts (CONFIRMED, DISPROVED,
// UNVERIFIED) used by the verify-step review.
const ctxReviewVerdictHelp = "CONFIRMED | DISPROVED | UNVERIFIED"

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

// reviewProtocolReminder is the fixed protocol text stored in every Review
// block: closed verdicts, evidence, and no self-approval.
func reviewProtocolReminder() string {
	return "Verify-step protocol: each finding ends as **CONFIRMED**, **DISPROVED** or\n" +
		"**UNVERIFIED**, with evidence (command output, file path). An independent\n" +
		"pass validates findings; the author of the phase does not self-approve."
}
