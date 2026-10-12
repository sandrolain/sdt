package cmd

import (
	"fmt"
	"strings"
	"testing"
)

func TestLintTextIsBoundedByDefaultAndReportsTheTotals(t *testing.T) {
	issues := make([]ctxLintIssue, 0, 65)
	for i := 0; i < 40; i++ {
		issues = append(issues, ctxLintIssue{Path: "a.md", Priority: ctxLintWarning, Message: "w"})
	}
	for i := 0; i < 25; i++ {
		issues = append(issues, ctxLintIssue{Path: "b.md", Priority: ctxLintSuggestionPriority, Message: "s"})
	}
	out := string(lintTextReport(issues, ctxLintDefaultLimit))

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != ctxLintDefaultLimit+1 {
		t.Fatalf("want %d issues plus one summary line, got %d lines", ctxLintDefaultLimit, len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "40 WARNING") ||
		!strings.Contains(lines[len(lines)-1], "25 SUGGESTION") ||
		!strings.Contains(lines[len(lines)-1], "65 total") {
		t.Errorf("summary must carry the complete totals: %q", lines[len(lines)-1])
	}
	if !strings.Contains(lines[len(lines)-1], "rerun with --limit 0") {
		t.Errorf("a withheld set must name the flag that prints it: %q", lines[len(lines)-1])
	}
}

func TestLintTextWithLimitZeroPrintsEveryFinding(t *testing.T) {
	issues := make([]ctxLintIssue, 0, 65)
	for i := 0; i < 65; i++ {
		issues = append(issues, ctxLintIssue{Path: "a.md", Priority: ctxLintWarning, Message: "w"})
	}
	out := string(lintTextReport(issues, 0))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 66 {
		t.Fatalf("--limit 0 must print all 65 issues plus the summary, got %d lines", len(lines))
	}
	if strings.Contains(lines[len(lines)-1], "rerun with") {
		t.Errorf("nothing was withheld, so the summary must not claim otherwise: %q", lines[len(lines)-1])
	}
}

func TestLintTextAlwaysReportsTotalsEvenWhenNothingIsWithheld(t *testing.T) {
	issues := []ctxLintIssue{{Path: "a.md", Priority: ctxLintCritical, Message: "c"}}
	out := string(lintTextReport(issues, ctxLintDefaultLimit))
	if !strings.Contains(out, "1 CRITICAL") || !strings.Contains(out, "1 total") {
		t.Errorf("a short run must still carry the totals: %q", out)
	}
}

func TestUnknownCommandDiagnosticNamesTheExecutableAndSuggests(t *testing.T) {
	exe := invokedName(rootCmd)
	err := commandDiagnostic(rootCmd, errUnknownCommand(`nope`, rootCmd.Name()))
	if err == nil {
		t.Fatal("want a diagnostic for an unknown command")
	}
	msg := err.Error()
	if !strings.Contains(msg, `unknown command "nope"`) {
		t.Errorf("missing the typed token: %q", msg)
	}
	if !strings.Contains(msg, exe) {
		t.Errorf("missing the resolved executable %q: %q", exe, msg)
	}
	if !strings.Contains(msg, "--help") {
		t.Errorf("missing the help pointer: %q", msg)
	}
}

func TestUnknownSubcommandSuggestsFromTheParent(t *testing.T) {
	err := commandDiagnostic(rootCmd, errUnknownCommand("srch", "sdt context"))
	if err == nil {
		t.Fatal("want a diagnostic for an unknown subcommand")
	}
	if !strings.Contains(err.Error(), "search") {
		t.Errorf("the suggestion must come from the parent command: %q", err)
	}
}

func TestUnknownFlagTokenIsNotReportedAsACommand(t *testing.T) {
	err := commandDiagnostic(rootCmd, errUnknownCommand("context lint --limit", "sdt"))
	if err == nil {
		t.Fatal("want a diagnostic for a dangling flag")
	}
	if strings.Contains(err.Error(), "unknown command") {
		t.Errorf("a flag token must not be reported as a command: %q", err)
	}
	if !strings.Contains(err.Error(), "--limit") {
		t.Errorf("the flag must be named: %q", err)
	}
}

func TestCommandDiagnosticPassesOtherErrorsThrough(t *testing.T) {
	if got := commandDiagnostic(rootCmd, nil); got != nil {
		t.Errorf("a clean run must report no error, got %v", got)
	}
	if got := commandDiagnostic(rootCmd, errOther{}); got == nil || !strings.Contains(got.Error(), "boom") {
		t.Errorf("an unrelated error must pass through untouched, got %v", got)
	}
}

type errOther struct{}

func (errOther) Error() string { return "boom" }

// errUnknownCommand renders the error cobra produces for an unknown command, so
// the diagnostic can be exercised without running the command tree.
func errUnknownCommand(typed, target string) error {
	return fmt.Errorf(`unknown command %q for %q`, typed, target)
}
