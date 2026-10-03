package research

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordFetchWritesRawAndProvenance(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)

	src := Source{CanonicalURL: "https://example.com/a/", Title: "A", ContentType: "text/markdown"}
	if err := r.RecordFetch(rawDir, src, []byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if len(r.Sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(r.Sources))
	}
	got := r.Sources[0]
	if got.Status != StatusFetched {
		t.Errorf("status = %q, want fetched", got.Status)
	}
	if got.SHA256 == "" || got.Bytes != int64(len("hello world")) {
		t.Errorf("provenance = %+v", got)
	}
	if got.CanonicalURL != "https://example.com/a" {
		t.Errorf("canonical URL not normalized: %q", got.CanonicalURL)
	}
	abs := filepath.Join(filepath.Dir(rawDir), filepath.FromSlash(got.RawPath))
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("raw payload = %q", data)
	}
	if got.RawPath == "" || !strings.HasPrefix(got.RawPath, RawDirName+"/") {
		t.Errorf("raw path must be run-relative under %s: %q", RawDirName, got.RawPath)
	}
}

func TestRecordFetchUpdatesExisting(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)
	r.AddSource(Source{CanonicalURL: "https://example.com/a", Status: StatusDiscovered})

	if err := r.RecordFetch(rawDir, Source{CanonicalURL: "https://example.com/a", Title: "A"}, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if len(r.Sources) != 1 {
		t.Fatalf("RecordFetch must update in place, got %d sources", len(r.Sources))
	}
	if r.Sources[0].Status != StatusFetched || r.Sources[0].Title != "A" {
		t.Errorf("existing source not updated: %+v", r.Sources[0])
	}
}

func TestRecordFailureKeepsSourceRetryable(t *testing.T) {
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://example.com/a", Status: StatusDiscovered})
	r.RecordFailure("https://example.com/a/", errors.New("boom"))
	s := r.Source("https://example.com/a")
	if s == nil {
		t.Fatal("source missing")
	}
	if s.Retries != 1 || s.Error != "boom" {
		t.Errorf("failure not recorded: %+v", s)
	}
	if s.Status != StatusDiscovered {
		t.Errorf("status = %q, want discovered (retryable)", s.Status)
	}
	r.RecordFailure("https://example.com/a", errors.New("again"))
	if s.Retries != 2 {
		t.Errorf("retries = %d, want 2", s.Retries)
	}
}

func TestRecordFailureAddsUnknownSource(t *testing.T) {
	r := fixedRun()
	r.RecordFailure("https://example.com/new", errors.New("nope"))
	if len(r.Sources) != 1 || r.Sources[0].Status != StatusDiscovered {
		t.Errorf("an unknown failed URL should be recorded as discovered: %+v", r.Sources)
	}
}

func TestSourceSlugIsStableAndSafe(t *testing.T) {
	slug := SourceSlug("https://example.com/docs/intro.html")
	if strings.ContainsAny(slug, "/:?&= ") {
		t.Errorf("slug must be filesystem-safe, got %q", slug)
	}
	if !strings.HasPrefix(slug, "intro-") {
		t.Errorf("slug should derive from the path basename, got %q", slug)
	}
	if SourceSlug("https://example.com/docs/intro.html") != slug {
		t.Error("slug must be deterministic")
	}
	if SourceSlug("https://example.com/docs/intro.html/") != slug {
		t.Error("trailing slash must not change the slug")
	}
	if SourceSlug("https://example.com/docs/other") == slug {
		t.Error("different URLs must get different slugs")
	}
	// A bare host still yields a non-empty slug.
	if s := SourceSlug("https://example.com"); s == "" || strings.Contains(s, "/") {
		t.Errorf("host-only slug = %q", s)
	}
}
