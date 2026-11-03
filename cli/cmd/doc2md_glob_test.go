package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/doc2md"
)

func TestDoc2mdDirGlobSelectsSubset(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.epub", "epub")
	writeCtxDoc(t, "context/ingestion/b.epub", "epub")
	writeCtxDoc(t, "context/ingestion/c.pdf", "pdf")
	logs := captureCmdErr(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# ok\n"}
	}, nil)

	out := string(execute(t, doc2mdCmd, nil, "--dir", "context/ingestion", "--glob", "*.epub", "--keep"))
	if !strings.Contains(out, "ok\tcontext/ingestion/a.epub") || !strings.Contains(out, "ok\tcontext/ingestion/b.epub") {
		t.Fatalf("the glob subset was not converted:\n%s", out)
	}
	if strings.Contains(out, "c.pdf") {
		t.Fatalf("a document outside the glob was converted:\n%s", out)
	}
	if !strings.Contains(logs.String(), "documents skipped by --glob") {
		t.Errorf("the skipped count was not reported: %s", logs.String())
	}
}

func TestDoc2mdDirGlobMatchingNothingIsClean(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.epub", "epub")
	logs := captureCmdErr(t)

	out := string(execute(t, doc2mdCmd, nil, "--dir", "context/ingestion", "--glob", "*.nothing", "--keep"))
	if strings.TrimSpace(out) != "" {
		t.Fatalf("a glob matching nothing must convert nothing, got:\n%s", out)
	}
	if !strings.Contains(logs.String(), "documents skipped by --glob") {
		t.Errorf("the skipped count was not reported: %s", logs.String())
	}
}

func TestDoc2mdGlobWithoutDirFails(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.epub", "epub")
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, doc2mdCmd, nil, "context/ingestion/a.epub", "--glob", "*.epub")
		return ""
	})
}
