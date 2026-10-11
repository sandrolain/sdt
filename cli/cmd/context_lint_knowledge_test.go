package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lintKnowledgeDoc writes a document with the given frontmatter body and returns
// every finding whose message contains one of substr, so a rule can be asserted
// without pinning the whole finding set of the file.
func lintKnowledgeDoc(t *testing.T, frontmatter, body, substr string) []ctxLintIssue {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.md")
	content := "---\n" + frontmatter + "---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	var matched []ctxLintIssue
	for _, issue := range lintDoc(path) {
		if strings.Contains(issue.Message, substr) {
			matched = append(matched, issue)
		}
	}
	return matched
}

func assertNoFinding(t *testing.T, issues []ctxLintIssue, rule string) {
	t.Helper()
	if len(issues) > 0 {
		t.Errorf("%s: expected no finding, got %d: %s", rule, len(issues), issues[0].Message)
	}
}

func assertFinding(t *testing.T, issues []ctxLintIssue, rule, priority string) {
	t.Helper()
	if len(issues) != 1 {
		t.Fatalf("%s: expected exactly 1 finding, got %d", rule, len(issues))
	}
	if issues[0].Priority != priority {
		t.Errorf("%s: priority = %s, want %s (%s)", rule, issues[0].Priority, priority, issues[0].Message)
	}
}

const maturitySubstr = "`maturity`"

// TestMaturityAxisIsAdditive locks the plan's core promise: the axis is
// orthogonal and optional, so a document that carries no `maturity` key is never
// flagged. Each case is a kind whose lifecycle status is unaffected by the axis.
func TestMaturityAxisIsAdditive(t *testing.T) {
	for _, fm := range []string{
		"kind: analysis\nsummary: s\nstatus: active\n",
		"kind: architecture\nsummary: s\nstatus: current\n",
		"kind: decision\nsummary: s\nstatus: accepted\n",
		"kind: notes\nsummary: s\n",
		// A status value that shares a name with a maturity stage is not a
		// maturity declaration: `status: superseded` alone must stay silent.
		"kind: decision\nsummary: s\nstatus: superseded\n",
	} {
		assertNoFinding(t, lintKnowledgeDoc(t, fm, "body\n", maturitySubstr), strings.Split(fm, "\n")[0])
	}
}

func TestMaturityVocabulary(t *testing.T) {
	for _, stage := range ctxMaturityValues {
		fm := "kind: analysis\nsummary: s\nmaturity: " + stage + "\n"
		if stage == ctxMaturitySuperseded {
			fm += "superseded_by: analysis/other.md\n"
		}
		assertNoFinding(t, lintKnowledgeDoc(t, fm, "body\n", maturitySubstr), stage)
	}
	assertFinding(t, lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: Canonical\n", "body\n", maturitySubstr), "capitalised stage", ctxLintWarning)
	assertFinding(t, lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: stable\n", "body\n", maturitySubstr), "invented stage", ctxLintWarning)
}

func TestMaturitySupersededNeedsSuccessor(t *testing.T) {
	assertFinding(t,
		lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: superseded\n", "body\n", "`superseded_by`"),
		"superseded without a successor", ctxLintWarning)
	assertNoFinding(t,
		lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: superseded\nsuperseded_by: analysis/other.md\n", "body\n", "`superseded_by`"),
		"superseded with a successor")
	// The successor is a document reference, so it is resolved like `supersedes`.
	broken := lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: superseded\nsuperseded_by: analysis/nope.md\n", "body\n", "broken superseded_by reference")
	assertFinding(t, broken, "unresolvable successor", ctxLintWarning)
}

func TestMaturityPromotionNeedsWarrant(t *testing.T) {
	for _, stage := range ctxMaturityWarranted {
		assertFinding(t,
			lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: "+stage+"\n", "body\n", "`warrant`"),
			stage+" without a warrant", ctxLintSuggestion)
		assertNoFinding(t,
			lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: "+stage+"\nwarrant: measured at commit abc123 over 3 sessions\n", "body\n", "`warrant`"),
			stage+" with a warrant")
	}
	// A lower stage needs no warrant.
	assertNoFinding(t,
		lintKnowledgeDoc(t, "kind: analysis\nsummary: s\nmaturity: provisional\n", "body\n", "`warrant`"),
		"provisional without a warrant")
}

// The maturity axis applies to every kind, not only analyses: a wiki page and a
// task file carry it too.
func TestMaturityIsCrossKind(t *testing.T) {
	for _, kind := range []string{"analysis", "architecture", "wiki", "decision", "notes", "tasks", "plan"} {
		assertFinding(t,
			lintKnowledgeDoc(t, "kind: "+kind+"\nsummary: s\nmaturity: stable\n", "body\n", maturitySubstr),
			kind, ctxLintWarning)
	}
}

const stampSubstr = "`verified_"

func TestVerifiedStampIsPaired(t *testing.T) {
	const fm = "kind: architecture\nsummary: s\n"
	assertNoFinding(t, lintKnowledgeDoc(t, fm, "body\n", stampSubstr), "no stamp")
	assertNoFinding(t, lintKnowledgeDoc(t,
		fm+"verified_at: 2026-10-09T20:33:17Z\nverified_against: refs/codebase/openrig@da230f5b\n",
		"body\n", stampSubstr), "both halves")
	assertFinding(t, lintKnowledgeDoc(t,
		fm+"verified_at: 2026-10-09T20:33:17Z\n", "body\n", stampSubstr), "date without a locator", ctxLintWarning)
	assertFinding(t, lintKnowledgeDoc(t,
		fm+"verified_against: refs/codebase/openrig@da230f5b\n", "body\n", stampSubstr), "locator without a date", ctxLintWarning)
}

// `updated` dates the edit and must not be read as a verification stamp: the two
// are orthogonal, so a document carrying only `updated` stays silent.
func TestVerifiedStampIsOrthogonalToUpdated(t *testing.T) {
	assertNoFinding(t, lintKnowledgeDoc(t,
		"kind: architecture\nsummary: s\nupdated: 2026-10-09T20:33:17Z\n", "body\n", stampSubstr), "updated only")
}

// The stamp is a locator, not a corpus document reference: a clone sha or an
// upstream URL must not be resolved against the context tree.
func TestVerifiedAgainstIsNotACorpusReference(t *testing.T) {
	assertNoFinding(t, lintKnowledgeDoc(t,
		"kind: architecture\nsummary: s\nverified_at: 2026-10-09T20:33:17Z\nverified_against: https://example.invalid/spec@v1\n",
		"body\n", "broken verified_against"), "external locator")
}

const rigorSubstr = "`rigor`"

func TestRigorDial(t *testing.T) {
	for _, dial := range ctxRigorValues {
		assertNoFinding(t, lintKnowledgeDoc(t, "kind: plan\nsummary: s\nrigor: "+dial+"\n", "body\n", rigorSubstr), dial)
	}
	assertNoFinding(t, lintKnowledgeDoc(t, "kind: plan\nsummary: s\n", "body\n", rigorSubstr), "no dial")
	assertFinding(t, lintKnowledgeDoc(t, "kind: plan\nsummary: s\nrigor: P5\n", "body\n", rigorSubstr), "out of range", ctxLintWarning)
	assertFinding(t, lintKnowledgeDoc(t, "kind: plan\nsummary: s\nrigor: p3\n", "body\n", rigorSubstr), "lowercase dial", ctxLintWarning)
	assertFinding(t, lintKnowledgeDoc(t, "kind: plan\nsummary: s\nrigor: high\n", "body\n", rigorSubstr), "free text", ctxLintWarning)
}

func TestReviewDisposition(t *testing.T) {
	const head = "kind: tasks\nsummary: s\nstatus: completed\n"
	body := func(disposition string) string {
		b := "\n## Phase 1\n\n- [x] a step <!-- c1 -->\n\n## Review\n\n### Findings\n\n- [ ] the gate passed — CONFIRMED <!-- c2 -->\n"
		if disposition != "" {
			b += "\n### Disposition\n\n" + disposition + "\n"
		}
		return b
	}
	assertNoFinding(t, lintKnowledgeDoc(t, head, body(""), "Disposition"), "absent disposition")
	for _, value := range ctxDispositionValues {
		assertNoFinding(t, lintKnowledgeDoc(t, head, body(value), "Disposition"), value)
	}
	assertNoFinding(t, lintKnowledgeDoc(t, head, body("CONTEXT-GAP — the file was not written yet"), "Disposition"), "value with prose")
	assertFinding(t, lintKnowledgeDoc(t, head, body("EVIDENCE-GAP"), "Disposition"), "invented disposition", ctxLintWarning)
}

// The disposition belongs to the `## Review` block: a heading elsewhere in the
// task file must not be read as a recorded disposition.
func TestReviewDispositionScopedToReviewBlock(t *testing.T) {
	content := "---\nkind: tasks\nsummary: s\n---\n\n## Phase 1\n\n- [x] a step <!-- c1 -->\n\n### Disposition\n\nEVIDENCE-GAP\n"
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	for _, issue := range lintDoc(path) {
		if strings.Contains(issue.Message, "Disposition") {
			t.Errorf("disposition read outside `## Review`: %s", issue.Message)
		}
	}
}

// Each new key is declared, so a document adopting the conventions is not told
// its own frontmatter is undeclared.
func TestKnowledgeKeysAreDeclared(t *testing.T) {
	fm := "kind: analysis\nsummary: s\nmaturity: established\nwarrant: measured\nverified_at: 2026-10-09T20:33:17Z\nverified_against: refs/x@abc123\n"
	assertNoFinding(t, lintKnowledgeDoc(t, fm, "body\n", "undeclared frontmatter key"), "analysis axes")
	fm = "kind: plan\nsummary: s\nrigor: P3\n"
	assertNoFinding(t, lintKnowledgeDoc(t, fm, "body\n", "undeclared frontmatter key"), "rigor")
	fm = "kind: analysis\nsummary: s\nmaturity: superseded\nsuperseded_by: analysis/other.md\n"
	assertNoFinding(t, lintKnowledgeDoc(t, fm, "body\n", "undeclared frontmatter key"), "superseded_by")
}