package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanWikiOnlyVerified(t *testing.T) {
	r, root := runWithMixedSources(t)
	plan := r.PlanWiki(RunDir(root, r.RunID), "brief", nil, 0)

	if len(plan.Pages) != 1 || plan.Pages[0].Action != "create" {
		t.Fatalf("plan = %+v", plan.Pages)
	}
	joined := strings.Join(plan.Refused, "\n")
	if !strings.Contains(joined, "not verified") {
		t.Errorf("an unverified/rejected source must be refused:\n%s", joined)
	}
}

func TestPlanWikiDedupAgainstExistingIsUpdate(t *testing.T) {
	r, root := runWithMixedSources(t)
	// The verified source's slug already exists.
	id := SourceSlug("https://example.com/verified")
	plan := r.PlanWiki(RunDir(root, r.RunID), "brief", []string{id}, 0)

	if len(plan.Pages) != 1 || plan.Pages[0].Action != "update" {
		t.Fatalf("existing id must yield an update: %+v", plan.Pages)
	}
}

func TestPlanWikiBudget(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)
	for _, u := range []string{"https://example.com/a", "https://example.com/b", "https://example.com/c"} {
		if err := r.RecordFetch(rawDir, Source{CanonicalURL: u, Title: strings.ToUpper(u)}, []byte("body")); err != nil {
			t.Fatal(err)
		}
		r.Sources[len(r.Sources)-1].Status = StatusVerified
	}
	plan := r.PlanWiki(RunDir(root, r.RunID), "brief", nil, 2)
	if len(plan.Pages) != 2 {
		t.Fatalf("budget 2 must cap pages at 2: %d\n%+v", len(plan.Pages), plan.Pages)
	}
	if !strings.Contains(strings.Join(plan.Refused, "\n"), "page budget 2 reached") {
		t.Errorf("refusal must name the budget: %v", plan.Refused)
	}
}

func TestPlanWikiPageHasEvidenceCitation(t *testing.T) {
	r, root := runWithMixedSources(t)
	plan := r.PlanWiki(RunDir(root, r.RunID), "", nil, 0)
	if len(plan.Pages) != 1 || len(plan.Pages[0].Evidence) != 1 {
		t.Fatalf("plan = %+v", plan.Pages)
	}
	if !strings.Contains(plan.Pages[0].Evidence[0], "@") {
		t.Errorf("evidence must be a cited reference: %v", plan.Pages[0].Evidence)
	}
}

func TestPlanWikiRenderIsReadOnlyText(t *testing.T) {
	r, root := runWithMixedSources(t)
	out := r.PlanWiki(RunDir(root, r.RunID), "why", nil, 0).RenderPlan()
	for _, want := range []string{"Wiki promotion plan", "brief: why", "refused"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

func TestRenderPageCarriesEvidenceAndDraftStatus(t *testing.T) {
	r, root := runWithMixedSources(t)
	plan := r.PlanWiki(RunDir(root, r.RunID), "", nil, 0)
	page := plan.Pages[0]
	src := *r.Source("https://example.com/verified")
	md := RenderPage(page, src, r.RunID, RefRelPath(r, src))

	for _, want := range []string{"kind: wiki", "id: " + page.ID, "status: draft", "## Claims", "refs/"} {
		if !strings.Contains(md, want) {
			t.Errorf("page missing %q:\n%s", want, md)
		}
	}
}

func TestRefRelPath(t *testing.T) {
	r := fixedRun()
	r.Objective = "web-capture-tooling"
	r.Created = "2026-10-04T11:30:00Z"
	rel := RefRelPath(r, Source{Title: "Intro", CanonicalURL: "https://example.com/intro.html"})
	if want := "refs/20261004-113000-web-capture-tooling/intro.md"; rel != want {
		t.Errorf("RefRelPath = %q, want %q", rel, want)
	}
}

func TestArchiveDirName(t *testing.T) {
	r := fixedRun()
	r.Objective = "web-capture-tooling"
	r.Created = "2026-10-04T11:30:00Z"
	if got := ArchiveDirName(r); got != "20261004-113000-web-capture-tooling" {
		t.Errorf("ArchiveDirName = %q", got)
	}
	if got := ArchiveRefDir(r); got != "refs/20261004-113000-web-capture-tooling" {
		t.Errorf("ArchiveRefDir = %q", got)
	}
}

func TestArchiveDirNameFallbackAndDefensive(t *testing.T) {
	r := fixedRun()
	r.Objective = "x"
	r.Created = "not-a-timestamp"
	got := ArchiveDirName(r)
	if !strings.HasSuffix(got, "-x") || strings.Contains(got, "//") {
		t.Errorf("fallback ArchiveDirName = %q, want a now-stamped ...-x", got)
	}

	// An absent objective and query never yields an empty path segment.
	empty := &Run{Created: "2026-10-04T11:30:00Z"}
	if want := "refs/20261004-113000-research"; ArchiveRefDir(empty) != want {
		t.Errorf("ArchiveRefDir = %q, want %q", ArchiveRefDir(empty), want)
	}
}

func TestResultSlug(t *testing.T) {
	if got := ResultSlug(Source{Title: "Clean Architecture"}); got != "clean-architecture" {
		t.Errorf("title slug = %q", got)
	}
	if got := ResultSlug(Source{CanonicalURL: "https://example.com/docs/intro.html"}); got != "intro" {
		t.Errorf("url fallback = %q", got)
	}
	if got := ResultSlug(Source{}); got == "" {
		t.Error("result slug must never be empty")
	}
}

func TestArchiveFileNameCollisionAndDistinctNames(t *testing.T) {
	r := fixedRun()
	r.Sources = []Source{
		{CanonicalURL: "https://a/x", Title: "Same", SHA256: strings.Repeat("a", 64)},
		{CanonicalURL: "https://b/y", Title: "Same", SHA256: strings.Repeat("b", 64)},
	}
	a := ArchiveFileName(r, r.Sources[0])
	b := ArchiveFileName(r, r.Sources[1])
	if a == b {
		t.Errorf("colliding result names must differ: %q / %q", a, b)
	}
	if a != "same-aaaaaaaa.md" || b != "same-bbbbbbbb.md" {
		t.Errorf("collision names = %q / %q", a, b)
	}

	// Distinct titles keep distinct, plain names (no hash suffix).
	distinct := fixedRun()
	distinct.Sources = []Source{
		{CanonicalURL: "https://a/x", Title: "Alpha"},
		{CanonicalURL: "https://a/y", Title: "Beta"},
	}
	if got := ArchiveFileName(distinct, distinct.Sources[0]); got != "alpha.md" {
		t.Errorf("distinct name = %q, want alpha.md", got)
	}
	if got := ArchiveFileName(distinct, distinct.Sources[1]); got != "beta.md" {
		t.Errorf("distinct name = %q, want beta.md", got)
	}
}

func TestArchiveSourceCopiesRawIntoRefs(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	r.Objective = "web-capture-tooling"
	r.Created = "2026-10-04T11:30:00Z"
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)
	if err := r.RecordFetch(rawDir, Source{CanonicalURL: "https://example.com/a", Title: "Intro"}, []byte("archived body")); err != nil {
		t.Fatal(err)
	}
	src := r.Sources[0]

	rel, err := r.ArchiveSource(root, src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "refs/20261004-113000-web-capture-tooling/intro.md"; rel != want {
		t.Fatalf("archive rel = %q, want %q", rel, want)
	}
	if r.ArchiveDir != "refs/20261004-113000-web-capture-tooling" {
		t.Errorf("ArchiveDir = %q", r.ArchiveDir)
	}
	abs := filepath.Join(root, "context", filepath.FromSlash(rel))
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("archived file missing: %v", err)
	}
	if string(data) != "archived body" {
		t.Errorf("archived body = %q", data)
	}
	// Idempotent: a second archive keeps the existing file and path.
	if rel2, err := r.ArchiveSource(root, src); err != nil || rel2 != rel {
		t.Errorf("second archive = %q, %v", rel2, err)
	}
}

func TestArchiveSourceRefusesForeignDirectory(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	r.Objective = "web-capture-tooling"
	r.Created = "2026-10-04T11:30:00Z"
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)
	if err := r.RecordFetch(rawDir, Source{CanonicalURL: "https://example.com/a", Title: "Intro"}, []byte("body")); err != nil {
		t.Fatal(err)
	}
	// A foreign directory already owns the target name.
	foreign := filepath.Join(root, "context", "refs", "20261004-113000-web-capture-tooling")
	if err := os.MkdirAll(foreign, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ArchiveSource(root, r.Sources[0]); err == nil {
		t.Fatal("a pre-existing foreign directory must be refused")
	}
	// Nothing was written into it.
	entries, _ := os.ReadDir(foreign)
	if len(entries) != 0 {
		t.Errorf("refusal must not write, found %d entries", len(entries))
	}
}

func TestArchiveSourceRejectsPayloadless(t *testing.T) {
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://example.com/x", Status: StatusVerified})
	if _, err := r.ArchiveSource(t.TempDir(), r.Sources[0]); err == nil {
		t.Error("archiving a source without a payload must fail")
	}
}

func TestWikiPagePath(t *testing.T) {
	got := WikiPagePath("/root", "backend/auth")
	if want := filepath.Join("/root", "context", "wiki", "backend", "auth.md"); got != want {
		t.Errorf("WikiPagePath = %q, want %q", got, want)
	}
}
