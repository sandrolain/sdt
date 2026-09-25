package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupContextProject(t *testing.T) string {
	t.Helper()
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	return dir
}

func writeCtxDoc(t *testing.T, rel, frontmatter string) {
	t.Helper()
	path := filepath.Join(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(frontmatter), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContextReindex(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/plan/planx.md", "---\nkind: plan\nsummary: A plan summary\ncreated_at: 2026-01-01T00:00:00Z\n---\nbody\n")
	out := execute(t, contextReindexCmd, nil)
	if !strings.Contains(string(out), "context/index.md") {
		t.Fatalf("expected index path in output: %s", out)
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "context/index.md"))
	content := string(idx)
	if !strings.Contains(content, "[[plan/planx.md]]") {
		t.Errorf("expected plan doc in index:\n%s", content)
	}
	if !strings.Contains(content, "A plan summary") {
		t.Errorf("expected summary in index:\n%s", content)
	}
}

func TestContextReindexObjectiveGroups(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/analysis/alpha.md", "---\nkind: analysis\nsummary: Alpha analysis\nobjective: memory-bench\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/beta.md", "---\nkind: analysis\nsummary: Beta analysis\nobjective: memory-bench\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/gam.md", "---\nkind: analysis\nsummary: Gamma analysis\nobjective: viewer-io\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/noobj.md", "---\nkind: analysis\nsummary: Ungrouped analysis\n---\nbody\n")
	writeCtxDoc(t, "context/proposals/prop.md", "---\nkind: proposal\nsummary: Proposal not bucketed\n---\nbody\n")
	execute(t, contextReindexCmd, nil)
	idx, _ := os.ReadFile(filepath.Join(dir, "context/index.md"))
	content := string(idx)
	for _, want := range []string{"#### memory-bench", "#### viewer-io", "[[analysis/alpha.md]]", "[[analysis/beta.md]]", "[[analysis/gam.md]]", "[[analysis/noobj.md]]", "[[proposals/prop.md]]"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in index:\n%s", want, content)
		}
	}
	if strings.Count(content, "[[analysis/alpha.md]]") != 1 {
		t.Errorf("bucketed analysis must appear exactly once:\n%s", content)
	}
	if strings.Count(content, "[[analysis/noobj.md]]") != 1 {
		t.Errorf("unbucketed analysis must stay in the general list:\n%s", content)
	}
	if i, j := strings.Index(content, "#### memory-bench"), strings.Index(content, "#### viewer-io"); i == -1 || j == -1 || i > j {
		t.Errorf("objective sections must be sorted lexicographically:\n%s", content)
	}
}

func TestContextReindexProposalAndPrompt(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/source.md", "---\nkind: analysis\nsummary: Source analysis\nobjective: test\nlinks: none\nstatus: active\n---\nbody\n")
	writeCtxDoc(t, "context/refs/search.md", "---\nkind: reference\nstatus: archived\nsummary: Search evidence\n---\nsource\n")
	writeCtxDoc(t, "context/proposals/proposal.md", "---\nkind: proposal\ntitle: Proposal\nsummary: Proposal summary\nstatus: review\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nlinks:\n  - analysis/source.md\nsources:\n  - refs/search.md\n---\nbody\n")
	writeCtxDoc(t, "context/prompts/search.md", "---\nkind: prompt\ntitle: Search prompt\nsummary: Prompt summary\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nderived_from:\n  - analysis/source.md\nsources:\n  - refs/search.md\n---\nbody\n")

	execute(t, contextReindexCmd, nil)
	idx, err := os.ReadFile(filepath.Join("context", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[[proposals/proposal.md]]", "Proposal summary", "[[prompts/search.md]]", "Prompt summary"} {
		if !strings.Contains(string(idx), want) {
			t.Errorf("expected %q in index:\n%s", want, idx)
		}
	}
	if out := execute(t, contextLintCmd, nil); len(out) != 0 {
		t.Fatalf("expected clean proposal/prompt lint, got:\n%s", out)
	}
}

func TestContextReindexResearch(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/prompts/drive.md", "---\nkind: prompt\ntitle: Drive\nsummary: Driving prompt\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nderived_from:\n  - analysis/source.md\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/source.md", "---\nkind: analysis\nsummary: Source analysis\nobjective: test\nlinks: none\nstatus: active\n---\nbody\n")
	writeCtxDoc(t, "context/refs/capture.md", "---\nkind: reference\nstatus: archived\nsummary: Raw capture\n---\nsource\n")
	writeCtxDoc(t, "context/research/backends.md", "---\nkind: research\ntitle: Vector backends\nsummary: Compared vector backends\nsubject: Which vector backend fits? \nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nlinks:\n  - analysis/source.md\nsources:\n  - prompts/drive.md\n  - refs/capture.md\nproject: p\n---\n## Findings\nbody\n")

	if tier := ctxTierForDir(sdtResearchDir); tier != ctxTierImportant {
		t.Fatalf("research tier = %q, want %q", tier, ctxTierImportant)
	}
	execute(t, contextReindexCmd, nil)
	idx, err := os.ReadFile(filepath.Join("context", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(idx), "[[research/backends.md]]") || !strings.Contains(string(idx), "Compared vector backends") {
		t.Errorf("expected research entry in index:\n%s", idx)
	}
	if out := execute(t, contextLintCmd, nil); len(out) != 0 {
		t.Fatalf("expected clean research lint, got:\n%s", out)
	}
}

func TestContextLintPromptProvenance(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/prompts/broken.md", "---\nkind: prompt\ntitle: Broken prompt\nsummary: Prompt summary\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nderived_from:\n  - analysis/missing.md\n---\nbody\n")
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "broken derived_from reference") {
		t.Fatalf("expected broken provenance issue, got:\n%s", out)
	}
}

func TestContextLintProposalDecisionArchitectureChain(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/source.md", "---\nkind: analysis\nsummary: Source analysis\nobjective: test\nlinks: none\nstatus: active\n---\nbody\n")
	writeCtxDoc(t, "context/proposals/decision.md", "---\nkind: proposal\ntitle: Decision proposal\nsummary: Decision proposal\nstatus: accepted\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nlinks:\n  - analysis/source.md\n---\n## Decision outcome\nArchitectural decision.\n")
	writeCtxDoc(t, "context/decisions/0002-decision.md", "---\nkind: decision\nnumber: 0002\ntitle: Decision\nsummary: Accepted decision\nstatus: accepted\ncreated: 2026-01-01T00:00:00Z\nlinks:\n  - proposals/decision.md\nproject: p\nsources:\n  - proposals/decision.md\n---\n## Decision\nUse the proposal.\n")
	writeCtxDoc(t, "context/architecture/decision.md", "---\nkind: architecture\nsummary: Current decision architecture\ncontext: Decision shape\nstatus: current\ncomponent: decision\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\nlinks:\n  - decisions/0002-decision.md\nproject: p\n---\n# Architecture\n")
	if out := execute(t, contextLintCmd, nil); len(out) != 0 {
		t.Fatalf("expected clean proposal/decision/architecture chain, got:\n%s", out)
	}
}

func TestContextReindexSkipUnchanged(t *testing.T) {
	setupContextProject(t)
	first := execute(t, contextReindexCmd, nil)
	second := execute(t, contextReindexCmd, nil)
	if strings.Contains(string(second), "(written)") && !strings.Contains(string(first), "(written)") {
		t.Errorf("expected second run skipped")
	}
	if !strings.Contains(string(first), "(written)") {
		t.Errorf("expected first run written: %s", first)
	}
}

func TestContextLintOversizedTaskFile(t *testing.T) {
	setupContextProject(t)
	var body strings.Builder
	body.WriteString("---\nkind: tasks\nsummary: big phase\nlinks:\n  - plan/planx.md\nsources:\n  - plan/planx.md\n---\n")
	for i := 1; i <= 11; i++ {
		body.WriteString("- [ ] step\n")
	}
	writeCtxDoc(t, "context/tasks/big.md", body.String())
	writeCtxDoc(t, "context/plan/planx.md", "---\nkind: plan\nsummary: plan\n---\nbody\n")
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), ctxLintSuggestion) || !strings.Contains(string(out), "task file has 11 checklist items") || !strings.Contains(string(out), "consider splitting the phase") {
		t.Errorf("expected a SUGGESTION for the oversized task file: %s", out)
	}
	if strings.Contains(string(out), `"CRITICAL"`) {
		t.Errorf("expected no CRITICAL issues: %s", out)
	}
	yamlOut := execute(t, contextLintCmd, nil, "--format", "yaml")
	if !strings.Contains(string(yamlOut), "SUGGESTION") {
		t.Errorf("expected the suggestion in YAML output: %s", yamlOut)
	}
}

func TestContextLintOversizedTaskFileBoundary(t *testing.T) {
	setupContextProject(t)
	var body strings.Builder
	body.WriteString("---\nkind: tasks\nsummary: ok phase\n---\n")
	for i := 1; i <= 10; i++ {
		body.WriteString("- [ ] step\n")
	}
	writeCtxDoc(t, "context/tasks/ok.md", body.String())
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), "consider splitting the phase") {
		t.Errorf("expected no suggestion at exactly 10 items: %s", out)
	}
}

func TestContextLintTaskFileStatusVocabulary(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/tasks/bad.md", "---\nkind: tasks\nsummary: bad status\nstatus: done\n---\n- [ ] step\n")
	writeCtxDoc(t, "context/tasks/nostatus.md", "---\nkind: tasks\nsummary: no status\n---\n- [ ] step\n")
	writeCtxDoc(t, "context/tasks/legacy.md", "---\nkind: tasks\nsummary: legacy active\nstatus: active\n---\n- [ ] step\n")
	writeCtxDoc(t, "context/tasks/new.md", "---\nkind: tasks\nsummary: new vocab\nstatus: in-progress\n---\n- [~] step\n")
	out := strings.TrimSpace(string(execute(t, contextLintCmd, nil, "--format", "json")))
	var issues []ctxLintIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("invalid lint JSON: %v\n%s", err, out)
	}
	statusFlagged := func(base, msg string) bool {
		for _, it := range issues {
			if filepath.Base(it.Path) == base && strings.Contains(it.Message, msg) {
				return true
			}
		}
		return false
	}
	if !statusFlagged("bad.md", "outside vocabulary") {
		t.Errorf("expected `done` flagged as outside vocabulary, got issues: %s", out)
	}
	if !statusFlagged("nostatus.md", "missing frontmatter") {
		t.Errorf("expected missing status flagged, got issues: %s", out)
	}
	if statusFlagged("legacy.md", "outside vocabulary") {
		t.Errorf("legacy `active` must not be flagged, got issues: %s", out)
	}
	if statusFlagged("new.md", "outside vocabulary") {
		t.Errorf("valid `in-progress` must not be flagged, got issues: %s", out)
	}
}

// lintIssueContains reports whether the JSON lint output contains an issue
// whose message carries the given substring.
func lintIssueContains(out, substr string) bool {
	var issues []ctxLintIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return false
	}
	for _, it := range issues {
		if strings.Contains(it.Message, substr) {
			return true
		}
	}
	return false
}

// TestContextLintStatusVocabularyAllKinds covers the generic per-type status
// check across several status-bearing kinds: valid vocabularies pass, an
// out-of-vocab value is a WARNING, and a missing status is flagged.
func TestContextLintStatusVocabularyAllKinds(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/plan/okp.md", "---\nkind: plan\nsummary: good\nstatus: active\n---\nbody\n")
	writeCtxDoc(t, "context/proposals/bad.md", "---\nkind: proposal\ntitle: Bad\nsummary: bad\nstatus: shipped\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/missing.md", "---\nkind: analysis\nsummary: none\nobjective: t\n---\nbody\n")
	writeCtxDoc(t, "context/architecture/good.md", "---\nkind: architecture\nsummary: good\nstatus: current\n---\nbody\n")
	out := strings.TrimSpace(string(execute(t, contextLintCmd, nil, "--format", "json")))
	if lintIssueContains(out, "outside vocabulary") == false {
		t.Errorf("expected out-of-vocab proposal flagged, got: %s", out)
	}
	if lintIssueContains(out, "missing frontmatter `status`") == false {
		t.Errorf("expected missing analysis status flagged, got: %s", out)
	}
	for _, base := range []string{"okp.md", "good.md"} {
		if strings.Contains(strings.ToLower(out), `"`+base+`": "frontmatter`) {
			t.Errorf("valid status in %s must not be flagged, got: %s", base, out)
		}
	}
}

// TestContextLintArchivedInPlaceHint covers the SUGGESTION when a
// status-bearing document carries `status: archived` outside context/archive/;
// documents under archive/ and non-archived statuses stay silent.
func TestContextLintArchivedInPlaceHint(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/kept.md", "---\nkind: analysis\nsummary: kept in place\nstatus: archived\n---\nbody\n")
	writeCtxDoc(t, "context/research/live.md", "---\nkind: research\nsummary: still live\nstatus: active\n---\nbody\n")
	out := strings.TrimSpace(string(execute(t, contextLintCmd, nil, "--format", "json")))
	if lintIssueContains(out, "status-only `archived` set in place") == false {
		t.Errorf("expected archived-in-place SUGGESTION, got: %s", out)
	}
	if lintIssueContains(out, "research/live.md") {
		t.Errorf("non-archived status must not be hinted, got: %s", out)
	}
}

// TestContextLintTimestampFormat covers decision D4: a present but
// non-RFC3339 `created`/`updated` is a WARNING, RFC3339 values pass, and
// missing fields are not flagged.
func TestContextLintTimestampFormat(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/badtime.md", "---\nkind: analysis\nsummary: bad timestamp\nobjective: t\nstatus: active\ncreated: 2026-01-01\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/goodtime.md", "---\nkind: analysis\nsummary: good timestamp\nobjective: t\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-02T00:00:00Z\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/notime.md", "---\nkind: analysis\nsummary: no timestamp\nobjective: t\nstatus: active\n---\nbody\n")
	out := strings.TrimSpace(string(execute(t, contextLintCmd, nil, "--format", "json")))
	if lintIssueContains(out, "does not parse as RFC3339 UTC") == false {
		t.Errorf("expected bad timestamp flagged, got: %s", out)
	}
	if lintIssueContains(out, "goodtime.md") {
		t.Errorf("valid RFC3339 timestamps must not be flagged, got: %s", out)
	}
	if lintIssueContains(out, "notime.md") {
		t.Errorf("missing timestamps must not be flagged, got: %s", out)
	}
}

func TestContextLintClean(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/plan/good.md", "---\nkind: plan\nsummary: Good plan\nlinks:\n  - notes/other.md\n---\nbody\n")
	writeCtxDoc(t, "context/notes/other.md", "---\nkind: notes\nsummary: Other\n---\nbody\n")
	idx := "---\nkind: index\nsummary: index\n"
	idx += "[[plan/good.md]]\n"
	idx += "[[notes/other.md]]\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), `"CRITICAL"`) {
		t.Errorf("expected no CRITICAL issues: %s", out)
	}
}

func TestContextLintMissingSummary(t *testing.T) {
	setupContextProject(t)
	path := "context/plan/nosummary.md"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nkind: plan\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextLintCmd, nil, "--format", "json")
		return ""
	})
}

func TestContextLintBrokenLink(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/plan/nosearch.md", "---\nkind: plan\nsummary: x\n[[missing-file]]\n---\nbody\n")
	idx := "---\nkind: index\nsummary: i\n"
	idx += "[[plan/nosearch.md]]\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "broken link") {
		t.Errorf("expected broken link warning: %s", out)
	}
}

func TestContextStatus(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextStatusCmd, nil)
	for _, typ := range []string{"architecture", "decisions", "plan", "tasks"} {
		if !strings.Contains(string(out), typ+":") {
			t.Errorf("expected type %q in status: %s", typ, out)
		}
	}
}

func TestContextTemplate(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextTemplateCmd, nil, "--type", "decision")
	if !strings.Contains(string(out), "decisions/") && !strings.Contains(string(out), "Decision Records") {
		t.Errorf("expected decision template content: %s", out)
	}
}

func TestContextTemplateUnknownType(t *testing.T) {
	setupContextProject(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextTemplateCmd, nil, "--type", "bogus")
		return ""
	})
}

func TestContextListArchitectureAndDecisions(t *testing.T) {
	dir := setupContextProject(t)
	//#nosec G306 -- user work file
	if err := os.WriteFile(filepath.Join(dir, "context/architecture/stack.md"), []byte("---\nkind: architecture\nsummary: stack\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	//#nosec G306 -- user work file
	if err := os.WriteFile(filepath.Join(dir, "context/decisions/0001-x.md"), []byte("---\nkind: decision\nnumber: 0001\nsummary: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextListCmd, nil, "--type", "architecture")
	if !strings.Contains(string(out), "stack.md") {
		t.Errorf("expected stack.md in architecture list: %s", out)
	}
	out = execute(t, contextListCmd, nil, "--type", "decisions")
	if !strings.Contains(string(out), "0001-x.md") {
		t.Errorf("expected 0001-x.md in decisions list: %s", out)
	}
}

func TestContextTaskPhaseFile(t *testing.T) {
	dir := setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	execute(t, contextTaskAddCmd, nil, "step exec", "--phase", "execution", "--plan", "custom")
	path := filepath.Join(dir, "context/tasks/20260806-070000-custom-phase-execution.md")
	got := mustReadFile(t, path)
	for _, want := range []string{"step exec", "summary: Task checklist for phase execution", "status: pending"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in task file:\n%s", want, got)
		}
	}
	for _, forbid := range []string{"links:", "sources:"} {
		if strings.Contains(got, forbid) {
			t.Errorf("standalone task file must not reference a plan:\n%s", got)
		}
	}
	out := execute(t, contextTaskListCmd, nil, "--phase", "execution", "--plan", "custom", "--format", "json")
	if !strings.Contains(string(out), "step exec") {
		t.Errorf("expected step in execution list: %s", out)
	}
}

func TestContextTaskArchivePhaseFile(t *testing.T) {
	dir := setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	execute(t, contextTaskAddCmd, nil, "one", "--phase", "verify", "--plan", "custom")
	out := execute(t, contextTaskArchiveCmd, nil, "--phase", "verify", "--plan", "custom")
	archivePath := strings.TrimSpace(string(out))
	if !strings.Contains(archivePath, filepath.Join("context", "archive")) {
		t.Errorf("expected archive path, got %q", out)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archived file at %s: %v", archivePath, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "context/tasks/20260806-070000-custom-phase-verify.md")); !os.IsNotExist(err) {
		t.Error("expected task file removed after archive")
	}
}

func TestContextQuestionsPath(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextPathCmd, nil, "--type", "questions", "--slug", "backend")
	if !strings.Contains(string(out), filepath.Join("context", "questions")) {
		t.Errorf("expected questions path, got %s", out)
	}
	if !strings.Contains(string(out), "-backend.md") {
		t.Errorf("expected slug in questions path, got %s", out)
	}
}

func TestContextQuestionsNew(t *testing.T) {
	dir := setupContextProject(t)
	execute(t, contextNewCmd, nil, "--type", "questions", "--slug", "open-api", "--input", "body")
	path := filepath.Join(dir, "context", "questions")
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("expected questions dir: %v", err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Name(), "open-api") {
		t.Fatalf("expected one questions file with slug, got %v", entries)
	}
	data, _ := os.ReadFile(filepath.Join(path, entries[0].Name()))
	if !strings.Contains(string(data), "kind: questions") {
		t.Errorf("expected kind: questions in doc:\n%s", data)
	}
}

func TestContextQuestionsTemplate(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextTemplateCmd, nil, "--type", "questions")
	if !strings.Contains(string(out), "kind: questions") {
		t.Errorf("expected questions template content: %s", out)
	}
	if !strings.Contains(string(out), "sources") {
		t.Errorf("expected sources field in questions template: %s", out)
	}
}

func TestContextStatusIncludesQuestions(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextStatusCmd, nil)
	if !strings.Contains(string(out), "questions:") {
		t.Errorf("expected questions type in status: %s", out)
	}
}

func TestContextReindexIncludesQuestions(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/questions/q.md", "---\nkind: questions\nsummary: An open question\n---\nbody\n")
	execute(t, contextReindexCmd, nil)
	idx, _ := os.ReadFile(filepath.Join(dir, "context/index.md"))
	if !strings.Contains(string(idx), "[[questions/q.md]]") {
		t.Errorf("expected questions doc in index:\n%s", idx)
	}
}

func TestContextLintSourcesResolves(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/analysis/base.md", "---\nkind: analysis\nsummary: base\nobjective: test\n---\n")
	writeCtxDoc(t, "context/questions/q.md", "---\nkind: questions\nsummary: q\nsources:\n  - analysis/base.md\n---\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), "broken source") {
		t.Errorf("expected no broken source warnings for valid reference: %s", out)
	}
}

func TestContextLintSourcesMissingOnDerived(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/questions/q.md", "---\nkind: questions\nsummary: q\n---\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "missing `sources`") {
		t.Errorf("expected missing sources warning on derived questions doc: %s", out)
	}
}

func TestContextLintSourcesBroken(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/questions/q.md", "---\nkind: questions\nsummary: q\nsources:\n  - analysis/does-not-exist.md\n---\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "broken source") {
		t.Errorf("expected broken source warning: %s", out)
	}
}

// ── objective group key lint ──────────────────────────────────────────────────

func TestContextLintObjectiveValid(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/analysis/ok.md", "---\nkind: analysis\nsummary: ok\nobjective: memory-1\nlinks: none\n---\nbody\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: no objective needed\n---\nbody\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, contextLintCmd, nil, "--format", "json"))
	if strings.Contains(out, "`objective`") || strings.Contains(out, ctxLintSuggestion) {
		t.Errorf("expected no objective issues for a valid key, got:\n%s", out)
	}
}

func TestContextLintObjectiveMissing(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/analysis/missing.md", "---\nkind: analysis\nsummary: no group\n---\nbody\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, contextLintCmd, nil, "--format", "json"))
	if !strings.Contains(out, ctxLintSuggestion) || !strings.Contains(out, "missing `objective`") {
		t.Errorf("expected a SUGGESTION for the missing objective, got:\n%s", out)
	}
	if strings.Contains(out, `"CRITICAL"`) {
		t.Errorf("missing objective must never be CRITICAL:\n%s", out)
	}
}

func TestContextLintObjectiveBadSlug(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/analysis/bad.md", "---\nkind: analysis\nsummary: bad\nobjective: My Objective!\n---\nbody\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, contextLintCmd, nil, "--format", "json"))
	if !strings.Contains(out, `"WARNING"`) || !strings.Contains(out, "must be a kebab-case slug") {
		t.Errorf("expected a WARNING for the malformed objective, got:\n%s", out)
	}
}
