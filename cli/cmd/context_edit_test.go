package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const editProbeDoc = "context/plan/planx.md"

// editProbeRef is the same document as addressed on the command line (the
// resolveContextDoc contract accepts a context/-relative path).
const editProbeRef = "plan/planx.md"

func editProbeFixture(t *testing.T) string {
	t.Helper()
	setupContextProject(t)
	writeCtxDoc(t, editProbeDoc,
		"---\nkind: plan\nuid: "+testUIDv7(9)+"\nsummary: A plan\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n## Phase 1\nold work\n\n## Phase 2\nmore\n")
	return editProbeRef
}

func TestContextEditWritesTheBodyAndKeepsFrontmatter(t *testing.T) {
	path := editProbeFixture(t)
	out := execute(t, contextEditCmd, nil, path, "--input", "## Phase 1\nnew work\n")

	if !strings.Contains(string(out), "-old work") || !strings.Contains(string(out), "+new work") {
		t.Errorf("edit must print the unified diff, got:\n%s", out)
	}
	got := readDoc(t, path)
	for _, keep := range []string{"kind: plan", "uid: " + testUIDv7(9), "summary: A plan", "created: 2026-01-01T00:00:00Z"} {
		if !strings.Contains(got, keep) {
			t.Errorf("frontmatter key %q was lost:\n%s", keep, got)
		}
	}
	if !strings.Contains(got, "## Phase 1\nnew work") || strings.Contains(got, "old work") {
		t.Errorf("body was not replaced:\n%s", got)
	}
	if !strings.Contains(got, "updated: ") {
		t.Errorf("updated must survive the edit:\n%s", got)
	}
}

func TestContextEditFileFlagFeedsTheBody(t *testing.T) {
	path := editProbeFixture(t)
	body := "## Phase 1\nfrom a file\n"
	bodyPath := "body.md"
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	execute(t, contextEditCmd, nil, path, "--file", bodyPath)
	if got := readDoc(t, path); !strings.Contains(got, "from a file") {
		t.Errorf("--file body was not written:\n%s", got)
	}
}

func TestContextEditDryRunWritesNothing(t *testing.T) {
	path := editProbeFixture(t)
	before := readDoc(t, path)
	out := execute(t, contextEditCmd, nil, path, "--input", "## Phase 1\npreview\n", "--dry-run")

	if !strings.Contains(string(out), "preview") {
		t.Errorf("dry run must show the proposed change, got:\n%s", out)
	}
	if after := readDoc(t, path); after != before {
		t.Errorf("--dry-run wrote to disk:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestContextEditRefusesAFrontmatterBlock(t *testing.T) {
	path := editProbeFixture(t)
	before := readDoc(t, path)
	input := "---\nkind: plan\nsummary: hijacked\n---\nbody\n"
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextEditCmd, nil, path, "--input", input)
		return ""
	})
	if after := readDoc(t, path); after != before {
		t.Errorf("a refused edit must not touch the file:\n%s", after)
	}
}

func TestContextEditRefusesAnEmptyBodyWithNoSource(t *testing.T) {
	path := editProbeFixture(t)
	before := readDoc(t, path)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextEditCmd, nil, path)
		return ""
	})
	if after := readDoc(t, path); after != before {
		t.Errorf("an empty edit must be refused, not applied:\n%s", after)
	}
}

func TestContextEditSectionScopedEdit(t *testing.T) {
	path := editProbeFixture(t)
	out := execute(t, contextEditCmd, nil, path, "--input", "## Phase 2\nrevisited\n", "--section", "phase-2")

	if !strings.Contains(string(out), "-more") || !strings.Contains(string(out), "+revisited") {
		t.Errorf("the diff must show the section change, got:\n%s", out)
	}
	got := readDoc(t, path)
	if !strings.Contains(got, "## Phase 1\nold work") {
		t.Errorf("the untouched section changed:\n%s", got)
	}
	if !strings.Contains(got, "## Phase 2\nrevisited") || strings.Contains(got, "\nmore\n") {
		t.Errorf("the targeted section was not replaced:\n%s", got)
	}
}

func TestContextEditSectionByHeading(t *testing.T) {
	path := editProbeFixture(t)
	execute(t, contextEditCmd, nil, path, "--input", "## Phase 1\nby heading\n", "--section", "Phase 1")
	if got := readDoc(t, path); !strings.Contains(got, "by heading") {
		t.Errorf("a section must be selectable by its heading:\n%s", got)
	}
}

func TestContextEditReportsNoChange(t *testing.T) {
	path := editProbeFixture(t)
	out := execute(t, contextEditCmd, nil, path, "--input", "## Phase 1\nold work\n\n## Phase 2\nmore\n")
	if got := strings.TrimSpace(string(out)); got != "no change" {
		t.Errorf("an identical body must report no change, got:\n%s", out)
	}
}

func TestContextEditJSONOutput(t *testing.T) {
	path := editProbeFixture(t)
	out := execute(t, contextEditCmd, nil, path, "--input", "## Phase 1\njson body\n", "--format", "json")
	var res contextEditResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("edit --format json is not readable: %v\n%s", err, out)
	}
	if !res.Changed || res.DryRun {
		t.Errorf("a real edit must report changed and not dry-run: %+v", res)
	}
	if res.Path == "" || !strings.Contains(res.Diff, "+json body") {
		t.Errorf("the result must carry the path and the diff: %+v", res)
	}
	// The text stream and the machine stream must agree about the change.
	if got := readDoc(t, path); !strings.Contains(got, "json body") {
		t.Errorf("the json path must still write the body:\n%s", got)
	}
}

func TestContextEditHelpCarriesTheDeclaredShape(t *testing.T) {
	for _, part := range []string{"WHEN —", "WHEN NOT —", "RETURNS —", "EXAMPLE —"} {
		if !strings.Contains(contextEditCmd.Long, part) {
			t.Errorf("edit help is missing the %s part", part)
		}
	}
	if contextEditCmd.Example == "" {
		t.Error("edit carries no example")
	}
}
