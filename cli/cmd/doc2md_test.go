package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/doc2md"
)

type doc2mdConvertFunc func(context.Context, doc2md.Options) doc2md.Result
type doc2mdToolsFunc func(context.Context, time.Duration) []doc2md.ToolStatus

func withDoc2mdSeams(t *testing.T, convert doc2mdConvertFunc, tools doc2mdToolsFunc) {
	t.Helper()
	prevConvert, prevTools := doc2mdConvert, doc2mdTools
	if convert != nil {
		doc2mdConvert = convert
	}
	if tools != nil {
		doc2mdTools = tools
	}
	t.Cleanup(func() { doc2mdConvert, doc2mdTools = prevConvert, prevTools })
}

func TestDoc2mdPrintsMarkdown(t *testing.T) {
	withDoc2mdSeams(t, func(_ context.Context, opts doc2md.Options) doc2md.Result {
		if opts.Input != "doc.pdf" {
			t.Errorf("input = %q", opts.Input)
		}
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# hi\n"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "doc.pdf")
	if string(out) != "# hi\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestDoc2mdJSONEnvelope(t *testing.T) {
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# hi\n"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "doc.pdf", "--format", "json")
	for _, want := range []string{`"status": "success"`, `"converter": "anydoc"`, `"attempts"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("json output missing %s: %s", want, out)
		}
	}
}

func TestDoc2mdFailureExits1(t *testing.T) {
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusFailed, Reason: "boom"}
	}, nil)

	shouldExitWithCode(t, 1, func() string {
		execute(t, doc2mdCmd, nil, "doc.pdf")
		return ""
	})
}

func TestDoc2mdNeedsOCRExits3(t *testing.T) {
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusNeedsOCR, Converter: "anydoc", Reason: "PDF needs OCR"}
	}, nil)

	shouldExitWithCode(t, 3, func() string {
		execute(t, doc2mdCmd, nil, "scan.pdf")
		return ""
	})
}

func TestDoc2mdUnavailableExits2(t *testing.T) {
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusUnavailable, Reason: "no runnable converter"}
	}, nil)

	shouldExitWithCode(t, 2, func() string {
		execute(t, doc2mdCmd, nil, "doc.pdf")
		return ""
	})
}

func TestDoc2mdMultipleInputsPreferWorstStatus(t *testing.T) {
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, opts doc2md.Options) doc2md.Result {
		if opts.Input == "scan.pdf" {
			return doc2md.Result{Status: doc2md.StatusNeedsOCR, Reason: "PDF needs OCR"}
		}
		return doc2md.Result{Status: doc2md.StatusSuccess, Markdown: "# ok\n"}
	}, nil)

	shouldExitWithCode(t, 3, func() string {
		execute(t, doc2mdCmd, nil, "doc.pdf", "scan.pdf")
		return ""
	})
}

func TestDoc2mdToolPinFlag(t *testing.T) {
	withDoc2mdSeams(t, func(_ context.Context, opts doc2md.Options) doc2md.Result {
		if opts.Tool != "markitdown" {
			t.Errorf("tool = %q", opts.Tool)
		}
		if opts.Timeout != 30*time.Second {
			t.Errorf("timeout = %s", opts.Timeout)
		}
		return doc2md.Result{Status: doc2md.StatusSuccess, Markdown: "# pinned\n"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "doc.docx", "--tool", "markitdown", "--timeout", "30s")
	if string(out) != "# pinned\n" {
		t.Errorf("stdout = %q", out)
	}
}

func captureCmdErr(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	previous := rootCmd.ErrOrStderr()
	rootCmd.SetErr(buf)
	t.Cleanup(func() { rootCmd.SetErr(previous) })
	return buf
}

// primeCmdFlags merges the persistent flags into rootCmd's local set so the
// shared flag helpers can look them up before the first Execute.
func primeCmdFlags() {
	_ = rootCmd.LocalFlags()
	_ = rootCmd.InheritedFlags()
}

// captureCmdIn makes the command tree's stdin reader deterministic: cobra's
// getIn walks up to the root when an intermediate command has no reader, and
// the root otherwise reads the real os.Stdin.
func captureCmdIn(t *testing.T, data []byte) {
	t.Helper()
	primeCmdFlags()
	previous := rootCmd.InOrStdin()
	rootCmd.SetIn(bytes.NewReader(data))
	t.Cleanup(func() { rootCmd.SetIn(previous) })
}

func TestDoc2mdUnsupportedInput(t *testing.T) {
	runInTempDir(t)
	logs := captureCmdErr(t)
	called := false
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		called = true
		return doc2md.Result{Status: doc2md.StatusSuccess, Markdown: "x"}
	}, nil)

	shouldExitWithCode(t, 1, func() string {
		execute(t, doc2mdCmd, nil, "image.png")
		return ""
	})
	if called {
		t.Error("converter ran for an unsupported input")
	}
	if !strings.Contains(logs.String(), "unsupported input") || !strings.Contains(logs.String(), ".pdf") {
		t.Errorf("log = %s", logs)
	}
}

// An epub must reach the converter chain: the registry used to refuse it before
// anydoc — the only converter that reads book formats — was ever asked.
func TestDoc2mdEpubReachesTheChain(t *testing.T) {
	runInTempDir(t)
	captureCmdErr(t)
	var got string
	withDoc2mdSeams(t, func(_ context.Context, opts doc2md.Options) doc2md.Result {
		got = opts.Input
		return doc2md.Result{Status: doc2md.StatusSuccess, Markdown: "converted"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "book.epub")
	if got != "book.epub" {
		t.Errorf("converter received %q, want the epub", got)
	}
	if !strings.Contains(string(out), "converted") {
		t.Errorf("markdown = %q", out)
	}
}

func TestDoc2mdKeepWritesCorpus(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "report.csv", "a,b\n1,2\n")
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# Report\n\nbody\n"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "report.csv", "--keep")
	rel := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if !strings.HasPrefix(rel, sdtConvertedDir+"/") || !strings.HasSuffix(rel, ".md") {
		t.Fatalf("kept path = %q", rel)
	}
	// The counts follow the path: the routing receipt's two numbers, measured on
	// the conversion. The fake's heading is level 1 — the document title, not a
	// section level — so the count is 0 by the doctrine's own definition.
	if !strings.Contains(string(out), "0 section-level headings, 3 lines\n") {
		t.Errorf("counts line missing after the path: %q", out)
	}
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source: report.csv", "converter: anydoc", "sha256:", "# Report"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q in kept file:\n%s", want, data)
		}
	}

	out2 := execute(t, doc2mdCmd, nil, "report.csv", "--keep")
	if strings.TrimSpace(strings.SplitN(string(out2), "\n", 2)[0]) != rel {
		t.Errorf("second keep path = %q, want %q", out2, rel)
	}
	again, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Error("second keep rewrote the file")
	}
}

func TestDoc2mdOutputFile(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "report.csv", "a\n")
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "docling", Markdown: "# R\n"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "report.csv", "--output-file", "out.md")
	if len(out) != 0 {
		t.Errorf("stdout should be empty, got %q", out)
	}
	data, err := os.ReadFile(filepath.Join(dir, "out.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "---\n") || !strings.Contains(string(data), "converter: docling") {
		t.Errorf("output file = %s", data)
	}
}

func TestDoc2mdKeepOutputFileConflict(t *testing.T) {
	runInTempDir(t)
	captureLogs(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, doc2mdCmd, nil, "report.csv", "--keep", "--output-file", "out.md")
		return ""
	})
}

func TestDoc2mdKeepJSONEnvelope(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "report.csv", "a\n")
	captureLogs(t)
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# R\n"}
	}, nil)

	out := execute(t, doc2mdCmd, nil, "report.csv", "--keep", "--format", "json")
	if !strings.Contains(string(out), `"kept_file"`) {
		t.Errorf("json = %s", out)
	}
	if strings.Contains(string(out), "# R") {
		t.Errorf("markdown should not be duplicated in the envelope: %s", out)
	}
}

func TestDoc2mdToolsTable(t *testing.T) {
	withDoc2mdSeams(t, nil, func(_ context.Context, timeout time.Duration) []doc2md.ToolStatus {
		if timeout != doc2md.DefaultTimeout {
			t.Errorf("timeout = %s", timeout)
		}
		return []doc2md.ToolStatus{
			{Name: "anydoc", Available: true, Version: "0.2.4"},
			{Name: "docling", Available: false, Reason: "broken install"},
		}
	})

	out := execute(t, doc2mdToolsCmd, nil)
	text := string(out)
	if !strings.Contains(text, "anydoc") || !strings.Contains(text, "available") || !strings.Contains(text, "0.2.4") {
		t.Errorf("table = %q", text)
	}
	if !strings.Contains(text, "docling") || !strings.Contains(text, "broken install") {
		t.Errorf("table = %q", text)
	}
}

func TestDoc2mdToolsJSON(t *testing.T) {
	withDoc2mdSeams(t, nil, func(_ context.Context, _ time.Duration) []doc2md.ToolStatus {
		return []doc2md.ToolStatus{{Name: "anydoc", Available: true, Version: "0.2.4"}}
	})

	out := execute(t, doc2mdToolsCmd, nil, "--format", "json")
	if !strings.Contains(string(out), `"name": "anydoc"`) || !strings.Contains(string(out), `"available": true`) {
		t.Errorf("json = %s", out)
	}
}

func TestDoc2mdListReadsFilesNotIndex(t *testing.T) {
	dir := runInTempDir(t)
	captureLogs(t)
	converted := filepath.Join(dir, sdtConvertedDir)
	writeTestFile(t, filepath.Join(converted, "a-deadbeefcafe.md"),
		"---\nsource: src/a.csv\nsha256: deadbeefcafe0000\nconverter: anydoc\nconverted_at: \"2026-10-01T09:00:00Z\"\nsummary: first\n---\n\nbody\n")
	writeTestFile(t, filepath.Join(converted, "index.md"), "| stale |\n")

	out := execute(t, doc2mdListCmd, nil)
	if !strings.Contains(string(out), "a-deadbeefcafe.md") || !strings.Contains(string(out), "first") {
		t.Errorf("list = %q", out)
	}
	if strings.Contains(string(out), "index.md") || strings.Contains(string(out), "stale") {
		t.Errorf("list included the index projection: %q", out)
	}

	jsonOut := execute(t, doc2mdListCmd, nil, "--format", "json")
	if !strings.Contains(string(jsonOut), `"file": "a-deadbeefcafe.md"`) || !strings.Contains(string(jsonOut), `"summary": "first"`) {
		t.Errorf("json = %s", jsonOut)
	}
}

func TestDoc2mdSetAndReindex(t *testing.T) {
	dir := runInTempDir(t)
	captureLogs(t)
	converted := filepath.Join(dir, sdtConvertedDir)
	writeTestFile(t, filepath.Join(converted, "a-deadbeefcafe.md"),
		"---\nsource: src/a.csv\nsha256: deadbeefcafe0000\nconverter: anydoc\nconverted_at: \"2026-10-01T09:00:00Z\"\n---\n\nbody\n")

	execute(t, doc2mdReindexCmd, nil)
	if _, err := os.Stat(filepath.Join(converted, "index.json")); err != nil {
		t.Fatalf("index.json not written: %v", err)
	}

	execute(t, doc2mdSetCmd, nil, "a-deadbeefcafe.md", "--summary", "enriched", "--link", "analysis/x.md")
	md, err := os.ReadFile(filepath.Join(converted, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "enriched") || !strings.Contains(string(md), "analysis/x.md") {
		t.Errorf("index.md not refreshed:\n%s", md)
	}

	doc, err := os.ReadFile(filepath.Join(converted, "a-deadbeefcafe.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "summary: enriched") || !strings.Contains(string(doc), "body") {
		t.Errorf("document = %s", doc)
	}
}

func TestDoc2mdSetRequiresSomething(t *testing.T) {
	runInTempDir(t)
	captureLogs(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, doc2mdSetCmd, nil, "a.md")
		return ""
	})
}

func TestDoc2mdReindexNoChange(t *testing.T) {
	dir := runInTempDir(t)
	captureLogs(t)
	converted := filepath.Join(dir, sdtConvertedDir)
	writeTestFile(t, filepath.Join(converted, "a-deadbeefcafe.md"),
		"---\nsource: src/a.csv\nsha256: deadbeefcafe0000\nconverter: anydoc\nconverted_at: \"2026-10-01T09:00:00Z\"\n---\n\nbody\n")

	first := string(execute(t, doc2mdReindexCmd, nil, "--format", "json"))
	if !strings.Contains(first, `"markdown": true`) || !strings.Contains(first, `"json": true`) {
		t.Errorf("first reindex = %s", first)
	}
	second := string(execute(t, doc2mdReindexCmd, nil, "--format", "json"))
	if !strings.Contains(second, `"markdown": false`) || !strings.Contains(second, `"json": false`) {
		t.Errorf("second reindex = %s", second)
	}
}

func TestDoc2mdDirJSONStatuses(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.pdf", "pdf")
	writeCtxDoc(t, "context/ingestion/b.epub", "epub")
	writeCtxDoc(t, "context/ingestion/notes.bin", "skip")
	withDoc2mdSeams(t, func(_ context.Context, opts doc2md.Options) doc2md.Result {
		if strings.HasSuffix(opts.Input, "b.epub") {
			return doc2md.Result{Status: doc2md.StatusNeedsOCR, Reason: "PDF needs OCR"}
		}
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# ok\n"}
	}, nil)

	var out string
	shouldExitWithCode(t, 3, func() string {
		out = string(execute(t, doc2mdCmd, nil, "--dir", "context/ingestion", "--format", "json"))
		return out
	})
	for _, want := range []string{`"input": "context/ingestion/a.pdf"`, `"status": "ok"`, `"input": "context/ingestion/b.epub"`, `"status": "needs_ocr"`} {
		if !strings.Contains(out, want) {
			t.Errorf("batch JSON is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "notes.bin") {
		t.Errorf("unsupported file was converted:\n%s", out)
	}
}

func TestDoc2mdDirRefusesTextWithoutKeep(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.pdf", "pdf")
	captureLogs(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, doc2mdCmd, nil, "--dir", "context/ingestion"))
	})
}

func TestDoc2mdDirTextStatusesWithKeep(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.pdf", "pdf")
	withDoc2mdSeams(t, func(_ context.Context, _ doc2md.Options) doc2md.Result {
		return doc2md.Result{Status: doc2md.StatusSuccess, Converter: "anydoc", Markdown: "# ok\n"}
	}, nil)

	out := string(execute(t, doc2mdCmd, nil, "--dir", "context/ingestion", "--keep"))
	if !strings.Contains(out, "ok\tcontext/ingestion/a.pdf") {
		t.Fatalf("batch text is missing the ok row:\n%s", out)
	}
	files, err := filepath.Glob("context/refs/converted/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Errorf("--keep wrote no converted artifact")
	}
}

func TestDoc2mdDirRefusesOutputFile(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/a.pdf", "pdf")
	captureLogs(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, doc2mdCmd, nil, "--dir", "context/ingestion", "--output-file", "out.md", "--format", "json"))
	})
}
