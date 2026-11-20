package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGuardedContextDocWriteRejectsStaleRevisionAndCanRetry(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "analysis", "x.md")
	original := []byte("---\nkind: analysis\nsummary: original\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\nbody\n")
	writeTestFile(t, filepath.Join(dir, path), string(original))
	staleRevision := contextDocumentRevision(original)

	current := bytes.Replace(original, []byte("summary: original"), []byte("summary: external"), 1)
	writeTestFile(t, filepath.Join(dir, path), string(current))
	proposed := strings.Replace(string(original), "body\n", "proposed\n", 1)
	err := guardedContextDocWrite(path, original, proposed, staleRevision)
	var conflict *contextMutationConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want revision conflict", err)
	}
	if conflict.CurrentRevision != contextDocumentRevision(current) {
		t.Fatalf("current revision = %s", conflict.CurrentRevision)
	}
	if !strings.Contains(conflict.Diff, "+proposed") || !strings.Contains(conflict.Diff, "-summary: external") {
		t.Fatalf("conflict diff did not preserve the proposal against current content:\n%s", conflict.Diff)
	}
	if got := mustReadFile(t, filepath.Join(dir, path)); got != string(current) {
		t.Fatalf("conflict overwrote the intervening edit:\n%s", got)
	}

	retry := strings.Replace(string(current), "body\n", "proposed\n", 1)
	if err := guardedContextDocWrite(path, current, retry, conflict.CurrentRevision); err != nil {
		t.Fatalf("retry with current revision: %v", err)
	}
	if got := mustReadFile(t, filepath.Join(dir, path)); !strings.Contains(got, "summary: external") || !strings.Contains(got, "proposed") {
		t.Fatalf("retry did not preserve both edits:\n%s", got)
	}
}

func TestContextSetExpectedRevisionReturnsConflictDecision(t *testing.T) {
	dir := runInTempDir(t)
	rel := "analysis/x.md"
	original := []byte(setDoc)
	writeTestFile(t, filepath.Join(dir, "context", rel), string(original))
	staleRevision := contextDocumentRevision(original)
	intervening := strings.Replace(string(original), "kept text", "external edit", 1)
	writeTestFile(t, filepath.Join(dir, "context", rel), intervening)

	var stderr bytes.Buffer
	previousErr := rootCmd.ErrOrStderr()
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() { rootCmd.SetErr(previousErr) })

	shouldExitWithCode(t, 4, func() string {
		return string(execute(t, contextSetCmd, nil, rel, "summary", "proposed", "--raw", "--expected-revision", staleRevision))
	})
	if got := readDoc(t, rel); !strings.Contains(got, "external edit") || strings.Contains(got, "summary: proposed") {
		t.Fatalf("stale set changed the file:\n%s", got)
	}
	for _, want := range []string{"sdt-conflict", "expected_revision=" + staleRevision, "current_revision=", "proposed diff:", "+summary: proposed"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("conflict output missing %q:\n%s", want, stderr.String())
		}
	}

	current := readDoc(t, rel)
	currentRevision := contextDocumentRevision([]byte(current))
	execute(t, contextSetCmd, nil, rel, "summary", "proposed", "--raw", "--expected-revision", currentRevision)
	if got := readDoc(t, rel); !strings.Contains(got, "summary: proposed") || !strings.Contains(got, "external edit") {
		t.Fatalf("retry did not preserve the intervening edit:\n%s", got)
	}
}

func TestGuardedContextDocWriteSerializesConcurrentWriters(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "plan", "plan.md")
	original := []byte("---\nkind: plan\nsummary: race\n---\nbody\n")
	writeTestFile(t, filepath.Join(dir, path), string(original))
	revision := contextDocumentRevision(original)

	start := make(chan struct{})
	results := make(chan error, 2)
	var writers sync.WaitGroup
	for _, replacement := range []string{"one", "two"} {
		writers.Add(1)
		go func(replacement string) {
			defer writers.Done()
			<-start
			proposed := strings.Replace(string(original), "body", replacement, 1)
			results <- guardedContextDocWrite(path, original, proposed, revision)
		}(replacement)
	}
	close(start)
	writers.Wait()
	close(results)

	successes, conflicts := 0, 0
	for err := range results {
		switch err {
		case nil:
			successes++
		default:
			var conflict *contextMutationConflict
			if !errors.As(err, &conflict) {
				t.Fatalf("unexpected writer error: %v", err)
			}
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d; want one each", successes, conflicts)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	retry := string(current) + "retry\n"
	if err := guardedContextDocWrite(path, current, retry, contextDocumentRevision(current)); err != nil {
		t.Fatalf("successful retry: %v", err)
	}
	got := mustReadFile(t, filepath.Join(dir, path))
	if !strings.Contains(got, "retry") || strings.Contains(got, "one\ntwo") {
		t.Fatalf("concurrent update or retry was lost:\n%s", got)
	}
}

func TestGuardedContextDocWriteKeepsLegacySchemaAndNoOp(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "notes", "legacy.md")
	original := []byte("---\nkind: notes\nsummary: legacy\n---\nbody\n")
	writeTestFile(t, filepath.Join(dir, path), string(original))
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := guardedContextDocWrite(path, original, string(original), ""); err != nil {
		t.Fatalf("no-op write: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("no-op touched the file: before=%s after=%s", before.ModTime(), after.ModTime())
	}

	proposed := string(original) + "next\n"
	if err := guardedContextDocWrite(path, original, proposed, ""); err != nil {
		t.Fatalf("legacy write without an expected revision: %v", err)
	}
	if got := mustReadFile(t, path); strings.Contains(got, "schema_version:") || !strings.Contains(got, "next") {
		t.Fatalf("legacy write changed schema or lost content:\n%s", got)
	}
}

func TestGuardedContextDocWriteRefusesUnsupportedSchema(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "notes", "future.md")
	original := []byte("---\nkind: notes\nschema_version: 2\nsummary: future\n---\nbody\n")
	writeTestFile(t, filepath.Join(dir, path), string(original))

	err := guardedContextDocWrite(path, original, string(original)+"changed\n", "")
	if err == nil || !strings.Contains(err.Error(), "unsupported schema_version 2") {
		t.Fatalf("error = %v, want unsupported schema", err)
	}
	if got := mustReadFile(t, filepath.Join(dir, path)); got != string(original) {
		t.Fatalf("unsupported schema was changed:\n%s", got)
	}
}

func TestGuardedContextDocWriteRefusesIntroducingUnsupportedSchema(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "notes", "future.md")
	original := []byte("---\nkind: notes\nsummary: current\n---\nbody\n")
	writeTestFile(t, filepath.Join(dir, path), string(original))
	proposed := strings.Replace(string(original), "summary: current", "schema_version: 2\nsummary: current", 1)

	err := guardedContextDocWrite(path, original, proposed, "")
	if err == nil || !strings.Contains(err.Error(), "proposed content: unsupported schema_version 2") {
		t.Fatalf("error = %v, want unsupported proposed schema", err)
	}
	if got := mustReadFile(t, filepath.Join(dir, path)); got != string(original) {
		t.Fatalf("unsupported proposed schema was written:\n%s", got)
	}
}

func TestGuardedContextDocWriteRefusesDuplicateSchemaVersion(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "notes", "duplicate.md")
	original := []byte("---\nkind: notes\nschema_version: 1\nschema_version: 1\nsummary: duplicate\n---\nbody\n")
	writeTestFile(t, filepath.Join(dir, path), string(original))

	err := guardedContextDocWrite(path, original, string(original)+"changed\n", "")
	if err == nil || !strings.Contains(err.Error(), "schema_version is duplicated") {
		t.Fatalf("error = %v, want duplicate schema error", err)
	}
	if got := mustReadFile(t, filepath.Join(dir, path)); got != string(original) {
		t.Fatalf("duplicate schema was changed:\n%s", got)
	}
}

func TestAtomicContextWriteFailurePreservesExistingDirectory(t *testing.T) {
	dir := runInTempDir(t)
	target := filepath.Join(dir, "context", "notes", "target.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "sentinel")
	writeTestFile(t, sentinel, "intact")
	if err := writeFileAtomic(target, []byte("replacement"), 0o644); err == nil {
		t.Fatal("rename over an existing directory unexpectedly succeeded")
	}
	if got := mustReadFile(t, sentinel); got != "intact" {
		t.Fatalf("failed atomic replacement damaged the existing destination: %q", got)
	}
}
