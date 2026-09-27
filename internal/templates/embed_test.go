package templates

import (
	"sort"
	"strings"
	"testing"
)

// TestListInstructions checks List returns the sorted .md.tmpl members of a
// template directory, filtering everything else.
func TestListInstructions(t *testing.T) {
	got := List("instructions")
	if len(got) == 0 {
		t.Fatal("List(instructions) returned no templates")
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("List(instructions) not sorted: %v", got)
	}
	for _, name := range got {
		if !strings.HasSuffix(name, ".md.tmpl") {
			t.Errorf("List(instructions) returned non-template %q", name)
		}
	}
	want := map[string]bool{"analysis.md.tmpl": false, "project.md.tmpl": false, "ui.md.tmpl": false}
	for _, name := range got {
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("List(instructions) missing %q", name)
		}
	}
}

// TestListSkipsNonTemplatesAndMissingDirs checks a directory with no .md.tmpl
// member (the embed root holds only directories) and a missing directory both
// yield an empty slice.
func TestListSkipsNonTemplatesAndMissingDirs(t *testing.T) {
	if got := List("."); len(got) != 0 {
		t.Errorf("List(.) = %v, want empty (top level holds directories only)", got)
	}
	if got := List("does/not/exist"); len(got) != 0 {
		t.Errorf("List(missing) = %v, want empty", got)
	}
}

// TestMustIsRenderForKnownTemplate checks Must succeeds where Render does and
// returns identical bytes.
func TestMustIsRenderForKnownTemplate(t *testing.T) {
	data := struct {
		Project string
		Group   string
	}{Project: "p", Group: "g"}
	rendered, err := Render("agents/instructions.md.tmpl", data, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := Must("agents/instructions.md.tmpl", data, nil); got != rendered {
		t.Error("Must output differs from Render output")
	}
}

// TestMustPanicsOnRenderError checks Must fails loud with the template name on
// the two Render error classes (missing template, execution error).
func TestMustPanicsOnRenderError(t *testing.T) {
	expectPanic := func(name string, data any) (recovered any) {
		defer func() { recovered = recover() }()
		_ = Must(name, data, nil)
		return nil
	}
	for name, tc := range map[string]struct {
		name string
		data any
		want string
	}{
		"missing template": {"agents/nope.md.tmpl", nil, "parse template agents/nope.md.tmpl"},
		"execution error":  {"agents/instructions.md.tmpl", struct{}{}, "execute template agents/instructions.md.tmpl"},
	} {
		t.Run(name, func(t *testing.T) {
			recovered := expectPanic(tc.name, tc.data)
			msg, ok := recovered.(string)
			if !ok || !strings.Contains(msg, tc.name) {
				t.Errorf("Must(%q) panicked with %v, want message containing %q", tc.name, recovered, tc.name)
			}
		})
	}
}

// TestRenderErrorPaths checks Render returns descriptive errors instead of
// panicking on parse and execution failures.
func TestRenderErrorPaths(t *testing.T) {
	if _, err := Render("instructions/nope.md.tmpl", nil, nil); err == nil {
		t.Error("Render of missing template must error")
	}
	// A struct without the fields the template reads triggers an execution error.
	if _, err := Render("agents/instructions.md.tmpl", struct{}{}, nil); err == nil {
		t.Error("Render with fields missing from data must error")
	}
}
