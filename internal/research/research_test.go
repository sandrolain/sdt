package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedRun() *Run {
	return NewRun("vector backends", "compare embedded stores", "", time.Unix(0, 0).UTC())
}

func TestNewRunIsTimeOrderedAndStamped(t *testing.T) {
	r := fixedRun()
	if r.RunID == "" {
		t.Fatal("expected a run id")
	}
	if r.Query != "vector backends" || r.Scope != "compare embedded stores" {
		t.Errorf("query/scope not preserved: %q / %q", r.Query, r.Scope)
	}
	if r.Sources == nil {
		t.Error("sources must be initialized, not nil (it serializes as [] not null)")
	}
	if r.Created != r.Updated {
		t.Errorf("created/updated must start equal, got %q / %q", r.Created, r.Updated)
	}
}

func TestAddSourceDeduplicatesByCanonicalURL(t *testing.T) {
	r := fixedRun()
	if !r.AddSource(Source{CanonicalURL: "https://example.com/a/"}) {
		t.Fatal("first add should succeed")
	}
	if r.AddSource(Source{CanonicalURL: "https://example.com/a"}) {
		t.Error("a trailing-slash variant is the same canonical URL and must dedup")
	}
	if !r.AddSource(Source{CanonicalURL: "https://example.com/b"}) {
		t.Error("a different URL must be added")
	}
	if got := len(r.Sources); got != 2 {
		t.Fatalf("sources = %d, want 2", got)
	}
	if r.Sources[0].Status != StatusDiscovered {
		t.Errorf("default status = %q, want %q", r.Sources[0].Status, StatusDiscovered)
	}
	if s := r.Source("https://example.com/a/"); s == nil || s.CanonicalURL != "https://example.com/a" {
		t.Errorf("Source() did not resolve the canonical form")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	r.Budget = Budget{MaxResults: 10, MaxCredits: 5}
	r.AddSource(Source{CanonicalURL: "https://example.com/a", Title: "A", Status: StatusFetched, SHA256: "abc", Bytes: 12})
	r.MarkCheckpoint("fetch")

	if err := r.Save(root); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Files exist where the contract says.
	for _, p := range []string{
		ManifestPath(root, r.RunID),
		filepath.Join(RunDir(root, r.RunID), ProvenanceName),
		filepath.Join(RunDir(root, r.RunID), RawDirName),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}

	got, err := Load(root, r.RunID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RunID != r.RunID || got.Query != r.Query {
		t.Errorf("round-trip identity mismatch: %+v", got)
	}
	if len(got.Sources) != 1 || got.Sources[0].Title != "A" {
		t.Errorf("source round-trip mismatch: %+v", got.Sources)
	}
	if got.Budget.MaxResults != 10 || got.Budget.MaxCredits != 5 {
		t.Errorf("budget round-trip mismatch: %+v", got.Budget)
	}
	if got.Checkpoint.Operation != "fetch" {
		t.Errorf("checkpoint = %q, want fetch", got.Checkpoint.Operation)
	}
	if got.CountByStatus(StatusFetched) != 1 {
		t.Errorf("CountByStatus(fetched) = %d, want 1", got.CountByStatus(StatusFetched))
	}
}

func TestSaveWritesProvenanceSidecar(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://example.com/a", Status: StatusVerified, SHA256: strings.Repeat("a", 64)})
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(RunDir(root, r.RunID), ProvenanceName))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{r.RunID, "vector backends", "https://example.com/a", StatusVerified} {
		if !strings.Contains(body, want) {
			t.Errorf("provenance missing %q:\n%s", want, body)
		}
	}
}

func TestLoadMissingRun(t *testing.T) {
	if _, err := Load(t.TempDir(), "nope"); err == nil {
		t.Fatal("expected an error for a missing run")
	}
}

func TestFindLatestReturnsNewest(t *testing.T) {
	root := t.TempDir()
	older := fixedRun()
	older.RunID = "00000000-0000-7000-8000-000000000001"
	newer := fixedRun()
	newer.RunID = "00000000-0000-7000-8000-000000000002"
	if err := older.Save(root); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := newer.Save(root); err != nil {
		t.Fatal(err)
	}
	got, err := FindLatest(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != newer.RunID {
		t.Errorf("FindLatest = %q, want %q", got, newer.RunID)
	}
}

func TestFindLatestEmptyRoot(t *testing.T) {
	got, err := FindLatest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("FindLatest on an empty root = %q, want empty", got)
	}
}

func TestHashFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, size, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// SHA-256("hello")
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if sum != want {
		t.Errorf("hash = %q, want %q", sum, want)
	}
	if size != 5 {
		t.Errorf("size = %d, want 5", size)
	}
}

func TestCanonicalURL(t *testing.T) {
	cases := map[string]string{
		"  https://example.com/a/  ": "https://example.com/a",
		"https://example.com/a":      "https://example.com/a",
		"":                           "",
	}
	for in, want := range cases {
		if got := CanonicalURL(in); got != want {
			t.Errorf("CanonicalURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewRunDerivesObjectiveFromQuery(t *testing.T) {
	r := NewRun("vector backends", "", "", time.Unix(0, 0).UTC())
	if r.Objective != "vector-backends" {
		t.Errorf("derived objective = %q, want vector-backends", r.Objective)
	}

	explicit := NewRun("ignored query", "", "refs-layout", time.Unix(0, 0).UTC())
	if explicit.Objective != "refs-layout" {
		t.Errorf("explicit objective = %q, want refs-layout", explicit.Objective)
	}
}

func TestObjectiveSlug(t *testing.T) {
	cases := []struct {
		name      string
		objective string
		query     string
		want      string
	}{
		{"explicit wins", "refs-layout", "ignored query", "refs-layout"},
		{"explicit sanitized", "Refs Layout!", "", "refs-layout"},
		{"query derived", "", "clean architecture notes", "clean-architecture-notes"},
		{"long slug bounded", "", strings.Repeat("a", 60), strings.Repeat("a", 48)},
		{"bound trims a trailing dash", "", strings.Repeat("a", 47) + " " + strings.Repeat("b", 10), strings.Repeat("a", 47)},
		{"sparse query falls back", "", "!!! ???", "research"},
		{"empty query falls back", "", "", "research"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ObjectiveSlug(&Run{Objective: tc.objective, Query: tc.query})
			if got != tc.want {
				t.Errorf("ObjectiveSlug(objective=%q, query=%q) = %q, want %q", tc.objective, tc.query, got, tc.want)
			}
		})
	}
	if got := ObjectiveSlug(nil); got != "research" {
		t.Errorf("ObjectiveSlug(nil) = %q, want research", got)
	}
}

func TestObjectiveRoundTrip(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	r.Objective = "web-capture-tooling"
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root, r.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Objective != "web-capture-tooling" {
		t.Errorf("objective round-trip = %q, want web-capture-tooling", got.Objective)
	}

	// A legacy manifest without `objective` loads with an empty objective (the
	// archive dir then derives from the query).
	legacy := filepath.Join(root, RunDirName, "legacy-run", ManifestName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "run_id: legacy-run\nquery: old query\ncreated: 2020-01-01T00:00:00Z\nupdated: 2020-01-01T00:00:00Z\nsources: []\n"
	if err := os.WriteFile(legacy, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := Load(root, "legacy-run")
	if err != nil {
		t.Fatal(err)
	}
	if old.Objective != "" {
		t.Errorf("legacy objective = %q, want empty", old.Objective)
	}
}
