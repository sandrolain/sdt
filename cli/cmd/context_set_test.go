package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

const setDoc = `---
kind: analysis
uid: 01a0eba1-c6cd-7e9b-8754-a8020ce74450
title: "My Title"
# a kept comment
objective: structured-data-access
status: active
created: 2026-09-29T05:27:23Z
updated: 2026-09-29T12:33:32Z
---

## Body

kept text
`

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("context/" + rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestContextSetScalarPreservesRest(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	execute(t, contextSetCmd, nil, "analysis/x.md", "summary", "a summary", "--raw")
	got := readDoc(t, "analysis/x.md")
	if !strings.Contains(got, "summary: a summary") {
		t.Errorf("summary not written:\n%s", got)
	}
	if !strings.Contains(got, "# a kept comment") {
		t.Errorf("comment lost:\n%s", got)
	}
	if !strings.Contains(got, "## Body\n\nkept text") {
		t.Errorf("body lost:\n%s", got)
	}
	if !strings.Contains(got, `title: "My Title"`) {
		t.Errorf("unrelated quoting changed:\n%s", got)
	}
}

func TestContextSetTypedValue(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	execute(t, contextSetCmd, nil, "analysis/x.md", "entities", "[a, b]")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "entities: [a, b]") {
		t.Errorf("list not written as flow list:\n%s", readDoc(t, "analysis/x.md"))
	}

	// A bare number stays a number (no quotes).
	execute(t, contextSetCmd, nil, "analysis/x.md", "weight", "3")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "weight: 3") {
		t.Errorf("number wrong:\n%s", readDoc(t, "analysis/x.md"))
	}
}

func TestContextSetListAppendRemove(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)
	execute(t, contextSetCmd, nil, "analysis/x.md", "categories", "[research]", "--force")
	execute(t, contextSetCmd, nil, "analysis/x.md", "categories", "cli", "--append")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "categories: [research, cli]") {
		t.Errorf("append failed:\n%s", readDoc(t, "analysis/x.md"))
	}
	execute(t, contextSetCmd, nil, "analysis/x.md", "categories", "cli", "--remove")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "categories: [research]") {
		t.Errorf("remove failed:\n%s", readDoc(t, "analysis/x.md"))
	}
}

// TestContextSetListAppendKeepsBlockSequence is the D1 guard: appending to a
// block-sequence list must keep every existing entry, not replace the list with
// the appended item.
func TestContextSetListAppendKeepsBlockSequence(t *testing.T) {
	runInTempDir(t)
	doc := "---\nkind: analysis\nsummary: block list\nlinks:\n  - a.md\n  - b.md\n  - c.md\n---\nbody\n"
	writeContextDoc(t, "analysis/x.md", doc)

	execute(t, contextSetCmd, nil, "analysis/x.md", "links", "d.md", "--append")
	got := readDoc(t, "analysis/x.md")
	for _, want := range []string{"a.md", "b.md", "c.md", "d.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("append dropped %q:\n%s", want, got)
		}
	}

	execute(t, contextSetCmd, nil, "analysis/x.md", "links", "b.md", "--remove")
	got = readDoc(t, "analysis/x.md")
	if strings.Contains(got, "b.md") {
		t.Errorf("remove failed:\n%s", got)
	}
	for _, want := range []string{"a.md", "c.md", "d.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("remove dropped %q:\n%s", want, got)
		}
	}
}

// TestContextSetAbsentListBecomesBlock guards the corpus default: appending to
// an absent key writes a block sequence, not an inline flow list.
func TestContextSetAbsentListBecomesBlock(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nsummary: absent\n---\nbody\n")

	execute(t, contextSetCmd, nil, "analysis/x.md", "links", "a.md", "--append")
	got := readDoc(t, "analysis/x.md")
	if !strings.Contains(got, "links:\n  - a.md") {
		t.Errorf("absent key not written as a block sequence:\n%s", got)
	}
}

// TestContextSetScalarListUpgraded guards the one-element scalar case: it is
// read as a one-item list and upgraded, not mistaken for an unreadable value.
func TestContextSetScalarListUpgraded(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nsummary: scalar list\nlinks: a.md\n---\nbody\n")

	execute(t, contextSetCmd, nil, "analysis/x.md", "links", "b.md", "--append")
	got := readDoc(t, "analysis/x.md")
	for _, want := range []string{"a.md", "b.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("scalar upgrade dropped %q:\n%s", want, got)
		}
	}
}

// TestContextSetUnreadableListRefuses is the refuse-don't-shrink guard: a
// non-empty value the shared reader cannot parse as a list is a hard error and
// the file keeps its bytes.
func TestContextSetUnreadableListRefuses(t *testing.T) {
	runInTempDir(t)
	const doc = "---\nkind: analysis\nsummary: map\nlinks:\n  a: b\n---\nbody\n"
	writeContextDoc(t, "analysis/x.md", doc)

	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextSetCmd, nil, "analysis/x.md", "links", "c.md", "--append")
		return ""
	})
	got := readDoc(t, "analysis/x.md")
	if got != doc {
		t.Errorf("refused write still changed the file:\n%s", got)
	}
}

func TestContextSetStatusValidation(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)
	captureCmdErr(t)

	shouldExitWithCode(t, 1, func() string {
		execute(t, contextSetCmd, nil, "analysis/x.md", "status", "banana")
		return ""
	})
	// Valid status writes.
	execute(t, contextSetCmd, nil, "analysis/x.md", "status", "draft")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "status: draft") {
		t.Errorf("valid status not written")
	}
}

func TestContextSetCLIOwnedKeyRefused(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)
	errs := captureCmdErr(t)

	shouldExitWithCode(t, 1, func() string {
		execute(t, contextSetCmd, nil, "analysis/x.md", "uid", "new-uid")
		return ""
	})
	if !strings.Contains(errs.String(), "sdt context uid") {
		t.Errorf("hint missing: %s", errs)
	}

	// --force allows it.
	execute(t, contextSetCmd, nil, "analysis/x.md", "uid", "forced", "--force")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "uid: forced") {
		t.Errorf("force did not write uid")
	}
}

func TestContextSetNestedKeyRejected(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextSetCmd, nil, "analysis/x.md", "meta.owner", "x")
		return ""
	})
}

func TestContextSetRefreshesUpdated(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	execute(t, contextSetCmd, nil, "analysis/x.md", "summary", "s", "--raw")
	got := readDoc(t, "analysis/x.md")
	if strings.Contains(got, "updated: 2026-09-29T12:33:32Z") {
		t.Errorf("updated not refreshed:\n%s", got)
	}
}

func TestContextSetUpdatedNotRefreshed(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	execute(t, contextSetCmd, nil, "analysis/x.md", "updated", "2030-01-01T00:00:00Z", "--raw")
	if !strings.Contains(readDoc(t, "analysis/x.md"), "updated: 2030-01-01T00:00:00Z") {
		t.Errorf("explicit updated was overwritten")
	}
}

func TestContextUnset(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	execute(t, contextUnsetCmd, nil, "analysis/x.md", "objective")
	got := readDoc(t, "analysis/x.md")
	if strings.Contains(got, "objective:") {
		t.Errorf("objective not removed:\n%s", got)
	}
	if !strings.Contains(got, "status: active") || !strings.Contains(got, "# a kept comment") {
		t.Errorf("unset damaged the block:\n%s", got)
	}
}

func TestContextSetYAMLValueReadsBackIdentical(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	// A value needing quoting (contains ": ") reads back identical through the
	// shared contextwiki reader, which strips one wrapping quote pair.
	value := "a quoted: value"
	execute(t, contextSetCmd, nil, "analysis/x.md", "summary", value, "--raw")

	got := readDoc(t, "analysis/x.md")
	back := contextwiki.FrontmatterField(got, "summary")
	if back != value {
		t.Errorf("read-back = %q, want %q", back, value)
	}

	// A value with a backslash/newline is fully escaped on write; the shared
	// line reader does not unescape, so assert the escaped text, not a decode.
	execute(t, contextSetCmd, nil, "analysis/x.md", "note", "line\nbreak", "--raw")
	line := contextwiki.FrontmatterField(readDoc(t, "analysis/x.md"), "note")
	if !strings.Contains(line, `\n`) {
		t.Errorf("newline not escaped on write: %q", line)
	}
}

// TestContextSetOneWriterGuard asserts that status/updated are written by the
// same vocabulary path as `status set` (they cannot disagree) and that no key
// is written by both the new engine and a legacy patcher.
func TestContextSetOneWriterGuard(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", setDoc)

	// status set must accept the same value set writes.
	execute(t, contextSetCmd, nil, "analysis/x.md", "status", "draft")
	viaSet := contextwiki.FrontmatterField(readDoc(t, "analysis/x.md"), "status")

	writeContextDoc(t, "analysis/y.md", setDoc)
	execute(t, contextStatusSetCmd, nil, "analysis/y.md", "--status", "draft")
	viaStatus := contextwiki.FrontmatterField(readDoc(t, "analysis/y.md"), "status")

	if viaSet != "draft" || viaStatus != "draft" {
		t.Errorf("status writers disagree: set=%q status=%q", viaSet, viaStatus)
	}

	// The legacy patcher keys must not overlap the CLI-owned guard set.
	for _, k := range []string{"uid", "analysis_id", "plan_id"} {
		if _, ok := ctxCLIOwnedKeys[k]; !ok {
			t.Errorf("CLI-owned key %q is no longer guarded", k)
		}
	}
}
