package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWithFetched(t *testing.T, body string) (*Run, string) {
	t.Helper()
	root := t.TempDir()
	r := fixedRun()
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)
	if err := r.RecordFetch(rawDir, Source{CanonicalURL: "https://example.com/a", Title: "A"}, []byte(body)); err != nil {
		t.Fatal(err)
	}
	return r, root
}

func TestVerifyAcceptsIntactSource(t *testing.T) {
	r, root := runWithFetched(t, "hello world")
	res := r.Verify(RunDir(root, r.RunID))

	if res.Checked != 1 || res.Verified != 1 || res.Rejected != 0 {
		t.Fatalf("result = %+v", res)
	}
	if r.Sources[0].Status != StatusVerified || r.Sources[0].Error != "" {
		t.Errorf("source = %+v", r.Sources[0])
	}
	if res.Failed() {
		t.Error("Failed() must be false on a clean run")
	}
}

func TestVerifyRejectsTamperedHash(t *testing.T) {
	r, root := runWithFetched(t, "hello world")
	// Change the bytes on disk without touching the manifest. RawPath already
	// includes the raw/ prefix and the .md suffix.
	abs := filepath.Join(RunDir(root, r.RunID), filepath.FromSlash(r.Sources[0].RawPath))
	if err := os.WriteFile(abs, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := r.Verify(RunDir(root, r.RunID))
	if res.Rejected != 1 || !strings.Contains(r.Sources[0].Error, "sha256 mismatch") {
		t.Fatalf("result = %+v, source error = %q", res, r.Sources[0].Error)
	}
	if r.Sources[0].Status != StatusRejected {
		t.Errorf("status = %q, want rejected", r.Sources[0].Status)
	}
}

func TestVerifyRejectsMissingRawFile(t *testing.T) {
	r, root := runWithFetched(t, "hello")
	abs := filepath.Join(RunDir(root, r.RunID), filepath.FromSlash(r.Sources[0].RawPath))
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}

	res := r.Verify(RunDir(root, r.RunID))
	if res.Rejected != 1 || !strings.Contains(r.Sources[0].Error, "raw payload missing") {
		t.Fatalf("result = %+v, error = %q", res, r.Sources[0].Error)
	}
}

func TestVerifyRejectsMissingRawPath(t *testing.T) {
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://example.com/a", Status: StatusFetched, SHA256: "abc"})
	res := r.Verify(t.TempDir())
	if res.Rejected != 1 || !strings.Contains(r.Sources[0].Error, "raw_path missing") {
		t.Fatalf("result = %+v, error = %q", res, r.Sources[0].Error)
	}
}

func TestVerifyRejectsEmptyPayload(t *testing.T) {
	root := t.TempDir()
	r := fixedRun()
	rawDir := filepath.Join(RunDir(root, r.RunID), RawDirName)
	// Write an empty file directly, then record it (RecordFetch hashes it).
	abs := filepath.Join(rawDir, "empty.md")
	if err := os.MkdirAll(rawDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r.Sources = append(r.Sources, Source{
		CanonicalURL: "https://example.com/empty",
		Status:       StatusFetched,
		RawPath:      RawDirName + "/empty.md",
		SHA256:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	})
	res := r.Verify(RunDir(root, r.RunID))
	if res.Rejected != 1 || !strings.Contains(r.Sources[0].Error, "empty") {
		t.Fatalf("result = %+v, error = %q", res, r.Sources[0].Error)
	}
}

func TestVerifySkipsDiscoveredSources(t *testing.T) {
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://example.com/a", Status: StatusDiscovered})
	res := r.Verify(t.TempDir())
	if res.Checked != 0 || res.Rejected != 0 {
		t.Errorf("discovered sources must not be checked: %+v", res)
	}
	if r.Sources[0].Status != StatusDiscovered {
		t.Errorf("discovered status must be untouched, got %q", r.Sources[0].Status)
	}
}

func TestVerifyIsDeterministicAndIdempotent(t *testing.T) {
	r, root := runWithFetched(t, "hello")
	first := r.Verify(RunDir(root, r.RunID))
	second := r.Verify(RunDir(root, r.RunID))
	if first.Verified != second.Verified || first.Rejected != second.Rejected || first.Checked != second.Checked {
		t.Errorf("verify must be idempotent:\nfirst  %+v\nsecond %+v", first, second)
	}
}
