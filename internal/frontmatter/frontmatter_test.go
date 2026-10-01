package frontmatter

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, content string) *Block {
	t.Helper()
	b, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return b
}

func TestParse(t *testing.T) {
	b := mustParse(t, "---\nkind: analysis\ntitle: \"x\"\n---\n\nbody\n")
	if b.YAML == "" {
		t.Fatal("empty YAML")
	}
	if v, ok := b.Key("kind"); !ok || v != "analysis" {
		t.Errorf("kind = %q ok=%v", v, ok)
	}
	if v, ok := b.Key("title"); !ok || v != "x" {
		t.Errorf("title = %q ok=%v", v, ok)
	}
	if _, ok := b.Key("missing"); ok {
		t.Error("missing key reported present")
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	if _, err := Parse("no frontmatter here\n"); err != ErrNoFrontmatter {
		t.Errorf("err = %v", err)
	}
	if _, err := Parse("---\nunterminated: true\n"); err != ErrNoFrontmatter {
		t.Errorf("unterminated err = %v", err)
	}
}

func TestParseMalformed(t *testing.T) {
	if _, err := Parse("---\n: : :\n---\n"); err == nil {
		t.Error("expected a parse error")
	}
}

func TestSetReplacesPreservingRest(t *testing.T) {
	content := "---\nkind: analysis\n# a comment\ntitle: \"My Title\"\nsummary: >\n  block\n  scalar\n---\n\nbody\n"
	b := mustParse(t, content)

	out, err := b.Set("kind", "plan", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := b.ReplaceBody(content, out)

	if !strings.Contains(doc, "kind: plan\n") {
		t.Errorf("kind not replaced:\n%s", doc)
	}
	if !strings.Contains(doc, "# a comment") {
		t.Errorf("comment lost:\n%s", doc)
	}
	if !strings.Contains(doc, "title: \"My Title\"") {
		t.Errorf("quoting lost:\n%s", doc)
	}
	if !strings.Contains(doc, "summary: >\n  block\n  scalar") {
		t.Errorf("block scalar modified:\n%s", doc)
	}
	if !strings.HasSuffix(doc, "---\n\nbody\n") {
		t.Errorf("body/fences modified:\n%q", doc)
	}
}

func TestSetKeyOrderPreserved(t *testing.T) {
	content := "---\na: 1\nb: 2\nc: 3\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Set("b", "9", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "a: 1\nb: 9\nc: 3" {
		t.Errorf("out = %q", out)
	}
}

func TestSetInsertsAfterNeighbour(t *testing.T) {
	content := "---\nkind: analysis\ntitle: x\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Set("summary", "s", []string{"kind", "title"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis\ntitle: x\nsummary: s" {
		t.Errorf("out = %q", out)
	}
}

func TestSetInsertsAfterSpecificNeighbour(t *testing.T) {
	content := "---\nkind: analysis\ntitle: x\nstatus: active\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Set("updated", "t", []string{"status"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis\ntitle: x\nstatus: active\nupdated: t" {
		t.Errorf("out = %q", out)
	}
}

func TestSetNestedKeyErrors(t *testing.T) {
	b := mustParse(t, "---\nkind: analysis\n---\nbody\n")
	if _, err := b.Set("meta.owner", "x", nil); err == nil || !strings.Contains(err.Error(), "nested") {
		t.Errorf("err = %v", err)
	}
}

func TestUnsetRemoves(t *testing.T) {
	content := "---\nkind: analysis\ntitle: x\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Unset("title")
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis" {
		t.Errorf("out = %q", out)
	}
}

func TestUnsetAbsentIsNoop(t *testing.T) {
	b := mustParse(t, "---\nkind: analysis\n---\nbody\n")
	out, err := b.Unset("missing")
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis" {
		t.Errorf("out = %q", out)
	}
}

func TestUnsetBlockValue(t *testing.T) {
	content := "---\nkind: analysis\ntopics:\n  - cli\n  - docs\nstatus: active\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Unset("topics")
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis\nstatus: active" {
		t.Errorf("out = %q", out)
	}
}

func TestCRLFRoundTrip(t *testing.T) {
	content := "---\r\nkind: analysis\r\ntitle: x\r\n---\r\n\r\nbody\r\n"
	b := mustParse(t, content)
	out, err := b.Set("kind", "plan", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := b.ReplaceBody(content, out)
	if !strings.Contains(doc, "kind: plan\r\n") {
		t.Errorf("CRLF not used on the written line:\n%q", doc)
	}
	if !strings.HasSuffix(doc, "---\r\n\r\nbody\r\n") {
		t.Errorf("body/fences modified:\n%q", doc)
	}
}

func TestAnchorAliasSurvives(t *testing.T) {
	content := "---\ndefaults: &plan\n  status: draft\nkind: analysis\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Set("kind", "plan", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := b.ReplaceBody(content, out)
	if !strings.Contains(doc, "defaults: &plan\n  status: draft") {
		t.Errorf("anchor lost:\n%s", doc)
	}
}

func TestScalar(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"", `""`},
		{"has: colon", `"has: colon"`},
		{"line\nbreak", `"line\nbreak"`},
		{`quote"inside`, `"quote\"inside"`},
		{"true", `"true"`},
		{"null", `"null"`},
		{"-leading", `"-leading"`},
		{"with # comment", `"with # comment"`},
		{"3", "3"},
	}
	for _, tc := range cases {
		if got := Scalar(tc.in); got != tc.want {
			t.Errorf("Scalar(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEscapedValueRoundTrips(t *testing.T) {
	content := "---\nkind: analysis\n---\nbody\n"
	b := mustParse(t, content)
	value := Scalar(`a "b"` + "\n" + `c`)
	out, err := b.Set("summary", value, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := b.ReplaceBody(content, out)
	b2, err := Parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := b2.Key("summary")
	if !ok {
		t.Fatalf("summary missing in:\n%s", doc)
	}
	if got != "a \"b\"\nc" {
		t.Errorf("round-trip = %q, want %q", got, "a \"b\"\nc")
	}
}

func TestReplaceBodyKeepsBodyBytes(t *testing.T) {
	content := "---\nkind: analysis\n---\n\n## Heading\n\nline\n"
	b := mustParse(t, content)
	out, err := b.Set("kind", "plan", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := b.ReplaceBody(content, out)
	if !strings.HasSuffix(doc, "\n## Heading\n\nline\n") {
		t.Errorf("body bytes changed:\n%q", doc)
	}
}

func TestInsertionAtEndWhenNoNeighbour(t *testing.T) {
	content := "---\nkind: analysis\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Set("summary", "s", []string{"nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis\nsummary: s" {
		t.Errorf("out = %q", out)
	}
}

func TestSetWithExistingSiblingBlock(t *testing.T) {
	// A key whose value is a block map must be replaced without eating the next key.
	content := "---\nkind: analysis\ntopics:\n  - cli\n  - docs\nstatus: active\n---\nbody\n"
	b := mustParse(t, content)
	out, err := b.Set("topics", "[]", []string{"kind"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "kind: analysis\ntopics: []\nstatus: active" {
		t.Errorf("out = %q", out)
	}
}

func TestSetIsIdempotent(t *testing.T) {
	content := "---\nkind: analysis\n---\nbody\n"
	b := mustParse(t, content)
	first, err := b.Set("summary", "s", nil)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := Parse(b.ReplaceBody(content, first))
	if err != nil {
		t.Fatal(err)
	}
	second, err := b2.Set("summary", "s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("not idempotent: %q vs %q", first, second)
	}
}

func TestInvalidKeyRejected(t *testing.T) {
	b := mustParse(t, "---\nkind: analysis\n---\nbody\n")
	for _, k := range []string{"", "a:b", "a\nb", "has space: x"} {
		if _, err := b.Set(k, "v", nil); err == nil {
			t.Errorf("Set(%q) accepted an invalid key", k)
		}
		if _, err := b.Unset(k); err == nil {
			t.Errorf("Unset(%q) accepted an invalid key", k)
		}
	}
}
