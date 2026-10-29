package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

// emptyBodyFile writes the explicitly empty body an accidental redirect
// produces, so the test and the incident use the same shape.
func emptyBodyFile(t *testing.T) string {
	t.Helper()
	name := "empty-body.md"
	if err := os.WriteFile(name, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

// TestContextEditRefusesAnEmptyBodyWithoutForce is the O2 gate: naming a body
// source was treated as consent to empty the page, which is exactly what a
// stray redirect does not mean. Both preview and real write are refused, and
// the file keeps every byte.
func TestContextEditRefusesAnEmptyBodyWithoutForce(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "dry-run"
		if dryRun {
			name = "write"
		}
		t.Run(name, func(t *testing.T) {
			path := editProbeFixture(t)
			before := readDoc(t, path)
			empty := emptyBodyFile(t)
			args := []string{path, "--file", empty}
			if dryRun {
				args = append(args, "--dry-run")
			}
			shouldExitWithCode(t, 1, func() string {
				execute(t, contextEditCmd, nil, args...)
				return ""
			})
			if after := readDoc(t, path); after != before {
				t.Errorf("a refused empty edit must not touch the file:\n%s", after)
			}
		})
	}
}

// TestContextEditForceEmptiesTheBody proves the refusal is a guard, not a
// removal of the capability: the explicit word still empties the body and keeps
// the frontmatter.
func TestContextEditForceEmptiesTheBody(t *testing.T) {
	path := editProbeFixture(t)
	execute(t, contextEditCmd, nil, path, "--file", emptyBodyFile(t), "--force")
	got := readDoc(t, path)
	if strings.Contains(got, "old work") || strings.Contains(got, "## Phase 1") {
		t.Errorf("--force must empty the body:\n%s", got)
	}
	for _, keep := range []string{"kind: plan", "uid: " + testUIDv7(9), "summary: A plan"} {
		if !strings.Contains(got, keep) {
			t.Errorf("--force lost frontmatter key %q:\n%s", keep, got)
		}
	}
}

// TestContextEditEmptySectionRemovalNeedsForce keeps the documented removal
// path reachable, behind the same explicit word: an empty section body is a
// destruction too.
func TestContextEditEmptySectionRemovalNeedsForce(t *testing.T) {
	path := editProbeFixture(t)
	before := readDoc(t, path)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextEditCmd, nil, path, "--section", "phase-2", "--input", "")
		return ""
	})
	if after := readDoc(t, path); after != before {
		t.Errorf("a refused section removal must not touch the file:\n%s", after)
	}
	execute(t, contextEditCmd, nil, path, "--section", "phase-2", "--input", "", "--force")
	got := readDoc(t, path)
	if strings.Contains(got, "## Phase 2") || strings.Contains(got, "more") {
		t.Errorf("--force must remove the whole section:\n%s", got)
	}
	if !strings.Contains(got, "## Phase 1\nold work") {
		t.Errorf("the untouched section changed:\n%s", got)
	}
}

// TestContextEditForceKeepsTheAcceptedShape proves --force changes nothing for
// a non-empty edit: the diff is printed, the body is written, and the JSON
// result keeps its fields.
func TestContextEditForceKeepsTheAcceptedShape(t *testing.T) {
	path := editProbeFixture(t)
	out := execute(t, contextEditCmd, nil, path, "--input", "## Phase 1\nforced\n", "--force", "--format", "json")
	var res contextEditResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("--force broke the JSON result: %v\n%s", err, out)
	}
	if !res.Changed || res.DryRun || !strings.Contains(res.Diff, "+forced") {
		t.Errorf("a forced non-empty edit changed shape: %+v", res)
	}
	if got := readDoc(t, path); !strings.Contains(got, "## Phase 1\nforced") {
		t.Errorf("a forced non-empty edit was not written:\n%s", got)
	}
}

// TestContextEditSectionRefusesABodyWithoutItsHeading is the O4 gate: the
// section range is heading-inclusive, so a body that does not begin with the
// heading would delete it. The refusal names the expected heading and writes
// nothing.
func TestContextEditSectionRefusesABodyWithoutItsHeading(t *testing.T) {
	path := editProbeFixture(t)
	before := readDoc(t, path)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextEditCmd, nil, path, "--section", "phase-2", "--input", "more\n")
		return ""
	})
	if after := readDoc(t, path); after != before {
		t.Errorf("a refused section body must not touch the file:\n%s", after)
	}
}

// TestContextEditSectionRefusesAWrongHeading proves --force is about emptiness
// only: a different heading text or level is still a structural error.
func TestContextEditSectionRefusesAWrongHeading(t *testing.T) {
	for _, body := range []string{"## Phase 1\nwrong text\n", "### Phase 2\nwrong level\n"} {
		t.Run(body, func(t *testing.T) {
			path := editProbeFixture(t)
			before := readDoc(t, path)
			shouldExitWithCode(t, 1, func() string {
				execute(t, contextEditCmd, nil, path, "--section", "phase-2", "--input", body, "--force")
				return ""
			})
			if after := readDoc(t, path); after != before {
				t.Errorf("a refused section body must not touch the file:\n%s", after)
			}
		})
	}
}

// TestContextEditSectionKeepsItsHeading proves every accepted replacement leaves
// exactly one heading, at its original position.
func TestContextEditSectionKeepsItsHeading(t *testing.T) {
	path := editProbeFixture(t)
	execute(t, contextEditCmd, nil, path, "--section", "phase-2", "--input", "## Phase 2\nrevisited\n")
	got := readDoc(t, path)
	if n := strings.Count(got, "## Phase 2"); n != 1 {
		t.Errorf("the heading appears %d times, want exactly 1:\n%s", n, got)
	}
	if !strings.Contains(got, "## Phase 2\nrevisited") {
		t.Errorf("the replacement body did not land under the heading:\n%s", got)
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

// TestContextEditAndGetShareTheReferenceGrammar is the O3 sentence proven at the
// command boundary: the same document is reachable by path, bare wiki name, wiki
// id and uid through both verbs, so they cannot disagree.
func TestContextEditAndGetShareTheReferenceGrammar(t *testing.T) {
	setupContextProject(t)
	writeTestFile(t, "context/wiki/architecture/anthology-saga.md", "---\nkind: wiki\nuid: "+testUIDv7(21)+
		"\nid: architecture/anthology-saga\nsummary: saga\nstatus: active\n---\n## Summary\noriginal\n")

	for _, ref := range []string{"wiki/architecture/anthology-saga.md", "architecture/anthology-saga", "anthology-saga", testUIDv7(21)} {
		out := strings.TrimSpace(string(execute(t, contextGetCmd, nil, ref, "id")))
		if out != "architecture/anthology-saga" {
			t.Errorf("context get %q resolved to %q", ref, out)
		}
	}

	for _, ref := range []string{"architecture/anthology-saga", testUIDv7(21)} {
		execute(t, contextEditCmd, nil, ref, "--input", "## Summary\nedited through "+ref+"\n")
		if got := readDoc(t, "wiki/architecture/anthology-saga.md"); !strings.Contains(got, "edited through "+ref) {
			t.Errorf("context edit %q did not reach the page:\n%s", ref, got)
		}
	}
}

// TestContextEditAndGetHelpNameTheReferenceForms keeps the declared grammar
// discoverable from --help, not only from the code.
func TestContextEditAndGetHelpNameTheReferenceForms(t *testing.T) {
	for _, cmd := range []*cobra.Command{contextEditCmd, contextGetCmd} {
		for _, part := range []string{"wiki page id", "uid"} {
			if !strings.Contains(cmd.Long, part) {
				t.Errorf("%s help does not name %q", cmd.Use, part)
			}
		}
	}
}

// TestContextEditAppendsInsteadOfReplacing proves the append half: the whole
// body changes only by the added lines, and the frontmatter survives.
func TestContextEditAppendsInsteadOfReplacing(t *testing.T) {
	path := editProbeFixture(t)
	execute(t, contextEditCmd, nil, path, "--append", "--input", "one more line\n")
	got := readDoc(t, path)
	if !strings.Contains(got, "## Phase 2\nmore\none more line\n") {
		t.Errorf("append must keep the body and add the line at its end:\n%s", got)
	}
	if !strings.Contains(got, "kind: plan") {
		t.Errorf("frontmatter lost on append:\n%s", got)
	}
	if !strings.Contains(string(execute(t, contextEditCmd, nil, path, "--append", "--input", "x\n", "--dry-run")), "+x") {
		t.Error("--dry-run must print the diff and write nothing")
	}
}

// TestContextEditAppendsToASection proves --section with --append adds after the
// section's last line, and does not require the heading a replacement needs.
func TestContextEditAppendsToASection(t *testing.T) {
	path := editProbeFixture(t)
	execute(t, contextEditCmd, nil, path, "--section", "Phase 1", "--append", "--input", "added to phase 1\n")
	got := readDoc(t, path)
	want := "## Phase 1\nold work\nadded to phase 1\n\n## Phase 2"
	if !strings.Contains(got, want) {
		t.Errorf("section append landed in the wrong place, want %q:\n%s", want, got)
	}
}

// TestContextEditAppendRefusals covers the two inputs that must not become a
// silent no-op: nothing to append, and a section that does not exist.
func TestContextEditAppendRefusals(t *testing.T) {
	path := editProbeFixture(t)
	for _, args := range [][]string{
		{path, "--append", "--input", "   \n"},
		{path, "--append", "--section", "Nope", "--input", "x\n"},
	} {
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, contextEditCmd, nil, args...))
		})
	}
}

// TestContextEditWritesTheScriptsIndex: the scripts index is a corpus file with
// no document kind, and registering a script used to be a host edit. The route
// resolves it, so the index is written by the CLI like any other body.
func TestContextEditWritesTheScriptsIndex(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/scripts/index.md",
		"---\nkind: scripts\nsummary: index\n---\n\n## Available scripts\n\n| name | purpose |\n|---|---|\n| old | row |\n")

	execute(t, contextEditCmd, nil, "scripts/index", "--input", "## Available scripts\n\n| name | purpose |\n|---|---|\n| new | row |\n")

	got := readDoc(t, "scripts/index.md")
	if !strings.Contains(got, "| new | row |") || strings.Contains(got, "| old | row |") {
		t.Errorf("the scripts index was not replaced:\n%s", got)
	}
	if !strings.Contains(got, "kind: scripts") {
		t.Errorf("the frontmatter was lost:\n%s", got)
	}
}
