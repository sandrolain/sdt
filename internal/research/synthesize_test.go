package research

import (
	"path/filepath"
	"strings"
	"testing"
)

func runWithMixedSources(t *testing.T) (*Run, string) {
	t.Helper()
	root := t.TempDir()
	r := fixedRun()
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)

	if err := r.RecordFetch(rawDir, Source{CanonicalURL: "https://example.com/verified", Title: "Verified"}, []byte("---\nurl: https://example.com/verified\n---\n\n# Verified\n\nVerified body.")); err != nil {
		t.Fatal(err)
	}
	r.Sources[len(r.Sources)-1].Status = StatusVerified

	if err := r.RecordFetch(rawDir, Source{CanonicalURL: "https://example.com/pending", Title: "Pending"}, []byte("# Pending\n\nPending body.")); err != nil {
		t.Fatal(err)
	}
	// Left as fetched (unverified).

	r.Sources = append(r.Sources, Source{CanonicalURL: "https://example.com/rejected", Status: StatusRejected})
	return r, root
}

func TestSynthesizeIncludesOnlyVerifiedByDefault(t *testing.T) {
	r, root := runWithMixedSources(t)
	d := r.Synthesize(RunDir(root, r.RunID), false, 0)

	if len(d.Excerpts) != 1 || d.Excerpts[0].CanonicalURL != "https://example.com/verified" {
		t.Fatalf("excerpts = %+v", d.Excerpts)
	}
	if d.Excerpts[0].Unverified {
		t.Error("the verified excerpt must not be flagged unverified")
	}
	// The rejected source is listed under Skipped.
	joined := strings.Join(d.Skipped, "\n")
	if !strings.Contains(joined, "rejected") {
		t.Errorf("skipped should mention the rejected source:\n%s", joined)
	}
	// The unverified (fetched) source is not included and not skipped-as-error.
	for _, e := range d.Excerpts {
		if e.CanonicalURL == "https://example.com/pending" {
			t.Error("an unverified source must be excluded by default")
		}
	}
}

func TestSynthesizeIncludesUnverifiedWhenAsked(t *testing.T) {
	r, root := runWithMixedSources(t)
	d := r.Synthesize(RunDir(root, r.RunID), true, 0)

	if len(d.Excerpts) != 2 {
		t.Fatalf("excerpts = %d, want 2: %+v", len(d.Excerpts), d.Excerpts)
	}
	var pending *Excerpt
	for i := range d.Excerpts {
		if d.Excerpts[i].CanonicalURL == "https://example.com/pending" {
			pending = &d.Excerpts[i]
		}
	}
	if pending == nil || !pending.Unverified {
		t.Errorf("the included unverified source must be flagged: %+v", pending)
	}
}

func TestSynthesizeCitationsAreDeterministic(t *testing.T) {
	r, root := runWithMixedSources(t)
	d1 := r.Synthesize(RunDir(root, r.RunID), false, 0)
	d2 := r.Synthesize(RunDir(root, r.RunID), false, 0)
	if d1.Excerpts[0].Citation != d2.Excerpts[0].Citation {
		t.Errorf("citation not deterministic: %q vs %q", d1.Excerpts[0].Citation, d2.Excerpts[0].Citation)
	}
	if !strings.HasPrefix(d1.Excerpts[0].Citation, "[") || !strings.Contains(d1.Excerpts[0].Citation, "@") {
		t.Errorf("citation shape unexpected: %q", d1.Excerpts[0].Citation)
	}
	if d1.Excerpts[0].SHA8 == "" || len(d1.Excerpts[0].SHA8) != 8 {
		t.Errorf("sha8 = %q, want 8 chars", d1.Excerpts[0].SHA8)
	}
}

func TestSynthesizeExcerptStripsFrontmatterAndTruncates(t *testing.T) {
	r, root := runWithMixedSources(t)
	d := r.Synthesize(RunDir(root, r.RunID), false, 0)
	body := d.Excerpts[0].Body
	if strings.Contains(body, "url: https://") {
		t.Errorf("frontmatter must be stripped from the excerpt:\n%s", body)
	}
	if !strings.Contains(body, "Verified body.") {
		t.Errorf("excerpt body missing content:\n%s", body)
	}

	trunc := r.Synthesize(RunDir(root, r.RunID), false, 1)
	if !trunc.Excerpts[0].Truncated {
		t.Error("a 1-line cap must mark the excerpt truncated")
	}
}

func TestSynthesizeSkipsMissingPayload(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://example.com/x", Status: StatusVerified, SHA256: "abc"})
	d := r.Synthesize(RunDir(root, r.RunID), false, 0)
	if len(d.Excerpts) != 0 {
		t.Errorf("a verified source without payload must be skipped: %+v", d.Excerpts)
	}
	if !strings.Contains(strings.Join(d.Skipped, "\n"), "no payload") {
		t.Errorf("skipped = %v", d.Skipped)
	}
}

func TestDraftRender(t *testing.T) {
	r, root := runWithMixedSources(t)
	md := r.Synthesize(RunDir(root, r.RunID), false, 0).Render()
	for _, want := range []string{"# Research draft:", "## Sources", "## Excerpts", "## Skipped", "https://example.com/verified"} {
		if !strings.Contains(md, want) {
			t.Errorf("render missing %q:\n%s", want, md)
		}
	}
}

func TestDraftRenderUnverifiedFlag(t *testing.T) {
	r, root := runWithMixedSources(t)
	md := r.Synthesize(RunDir(root, r.RunID), true, 0).Render()
	if !strings.Contains(md, "UNVERIFIED") {
		t.Errorf("an unverified excerpt must be flagged in the render:\n%s", md)
	}
}
