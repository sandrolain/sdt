package templates

import (
	"encoding/xml"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestDocumentedInstructionPathsResolve is the documented-path gate: a module or
// a skill that points at `context/instructions/<name>.md` names a module that
// ships, and a `context/instructions/<name>.md` in a shipped skill names one too.
// A dangling cross-reference sends a reader to a file that does not exist, and
// nothing else in the corpus reads those pointers.
func TestDocumentedInstructionPathsResolve(t *testing.T) {
	known := map[string]bool{}
	for _, tmpl := range List("instructions") {
		known[strings.TrimSuffix(tmpl, ".md.tmpl")] = true
	}
	if len(known) == 0 {
		t.Fatal("no instruction templates listed")
	}

	ref := regexp.MustCompile("`context/instructions/([a-z0-9-]+)\\.md`")
	check := func(name, body string) {
		for _, m := range ref.FindAllStringSubmatch(body, -1) {
			if !known[m[1]] {
				t.Errorf("%s points at context/instructions/%s.md, which no template ships", name, m[1])
			}
		}
	}

	for _, tmpl := range List("instructions") {
		check(tmpl, Must("instructions/"+tmpl, nil, nil))
	}
	for _, dir := range Dirs("skills") {
		for _, tmpl := range ListFiles("skills/" + dir) {
			check("skill "+dir, Must("skills/"+dir+"/"+tmpl, nil, nil))
		}
	}
}

// TestVectorSvgSkeletonPassesItsOwnCheck is the exemplar gate: the SVG the
// format module ships as its skeleton satisfies the rules the module states —
// a viewBox, no fixed root size, an accessible name and a description.
func TestVectorSvgSkeletonPassesItsOwnCheck(t *testing.T) {
	body := Must("instructions/vector-svg.md.tmpl", nil, nil)
	start := strings.Index(body, "```svg\n")
	if start < 0 {
		t.Fatal("vector-svg.md ships no svg example")
	}
	rest := body[start+len("```svg\n"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("the svg example is not closed")
	}
	example := rest[:end]

	var root struct {
		XMLName  xml.Name `xml:"svg"`
		ViewBox  string   `xml:"viewBox,attr"`
		Width    string   `xml:"width,attr"`
		Height   string   `xml:"height,attr"`
		Role     string   `xml:"role,attr"`
		Labelled string   `xml:"aria-labelledby,attr"`
		Titles   []struct {
			ID string `xml:"id,attr"`
		} `xml:"title"`
		Descs []struct {
			ID string `xml:"id,attr"`
		} `xml:"desc"`
	}
	if err := xml.Unmarshal([]byte(example), &root); err != nil {
		t.Fatalf("the skeleton is not well-formed XML: %v", err)
	}
	if root.ViewBox == "" {
		t.Error("the skeleton declares no viewBox")
	}
	if root.Width != "" || root.Height != "" {
		t.Error("the skeleton fixes a root size, which the scaling rule forbids")
	}
	if root.Role != "img" || root.Labelled == "" {
		t.Error("the skeleton is not named for assistive tech")
	}
	ids := map[string]bool{}
	for _, title := range root.Titles {
		ids[title.ID] = true
	}
	for _, desc := range root.Descs {
		ids[desc.ID] = true
	}
	var missing []string
	for _, want := range strings.Fields(root.Labelled) {
		if !ids[want] {
			missing = append(missing, want)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("aria-labelledby names ids the skeleton does not carry: %v", missing)
	}
}
