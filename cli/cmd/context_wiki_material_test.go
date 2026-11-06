package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// materialPage is a wiki page with two claims: the fixture a material pins.
const materialPage = `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: fixture
created: 2026-10-01T00:00:00Z
updated: 2026-10-20T10:00:00Z
project: p
---

## Summary

A page.

## Claims

1. {#claim-1} <!-- c-aaaaaaaa --> first fact (refs/a.md@aaaaaaaa:1-2)
2. {#claim-2} <!-- c-bbbbbbbb --> second fact (refs/a.md@aaaaaaaa:3-4)
`

// writeMaterialPage writes the fixture page under the temp corpus.
func writeMaterialPage(t *testing.T, id string) {
	t.Helper()
	writeCtxDoc(t, "context/wiki/"+id+".md", strings.Replace(materialPage, "architecture/alpha", id, 1))
}

// liveDigests reads the claim digests the page carries now, the value a material's
// header must pin to be fresh.
func liveDigests(t *testing.T, id string) map[string]string {
	t.Helper()
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		t.Fatalf("load wiki: %v", err)
	}
	page := bld.Resolve(id)
	if page == nil {
		t.Fatalf("page %q not found", id)
	}
	out := map[string]string{}
	for _, claim := range page.Claims() {
		out[claim.ID] = claim.Digest
	}
	return out
}

// materialHeaderLine renders the one line a material carries.
func materialHeaderLine(concept string, pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, pair[0]+"@"+pair[1])
	}
	return fmt.Sprintf("<!-- material: concept=wiki/%s; syllabus=syllabi/architect; lang=it; "+
		"classification=undeclared; claims=%s -->", concept, strings.Join(parts, ","))
}

// writeMaterial writes one material under the lane and returns its path.
func writeMaterial(t *testing.T, rel, header, body string) string {
	t.Helper()
	path := "context/renders/materials/software-architect/" + rel
	writeCtxDoc(t, path, header+"\n\n"+body)
	return path
}

// materialBody is the smallest prose that proves a binding: one gloss marker. The
// header tests use it so their fixture is fresh and the prose check is not the
// subject.
func materialBody() string {
	return "# Guida\n\nTesto.\n<!-- gloss=termine -->\n"
}

// TestWikiMaterialFreshAndStale proves the header is the identity: a matching pair
// set is fresh, a moved digest is stale, and the lane is named in the text report.
func TestWikiMaterialFreshAndStale(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	fresh := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]}, {"c-bbbbbbbb", digests["c-bbbbbbbb"]},
	})
	writeMaterial(t, "architecture-measurement.it.md", fresh, materialBody())
	stale := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]}, {"c-bbbbbbbb", "000000000000"},
	})
	writeMaterial(t, "alpha.it.md", stale, materialBody())

	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json"))
	byPath := map[string]renderRow{}
	for _, row := range rows {
		byPath[row.Path] = row
	}
	good := byPath["context/renders/materials/software-architect/architecture-measurement.it.md"]
	if good.Status != renderFresh || good.Lane != materialLane {
		t.Errorf("a matching header is fresh in the material lane, got %+v", good)
	}
	bad := byPath["context/renders/materials/software-architect/alpha.it.md"]
	if bad.Status != renderStale || !strings.Contains(bad.Detail, "moved") {
		t.Errorf("a moved digest is stale with its count, got %+v", bad)
	}

	out := string(execute(t, contextWikiRenderCmd, nil))
	if !strings.Contains(out, "fresh material architecture/alpha") ||
		!strings.Contains(out, "stale material architecture/alpha (1 claim(s) moved)") {
		t.Errorf("the text report must name the lane and the reason:\n%s", out)
	}
}

// TestWikiMaterialMissingOrphanAndUnreadableHeader covers the other three
// outcomes and the header that cannot be read.
func TestWikiMaterialMissingOrphanAndUnreadableHeader(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeMaterialPage(t, "architecture/alpha")
	writeMaterial(t, "alpha.it.md", materialHeaderLine("architecture/alpha",
		[][2]string{{"c-zzzzzzzz", "111111111111"}}), materialBody())
	writeMaterial(t, "gone.it.md", materialHeaderLine("architecture/gone",
		[][2]string{{"c-aaaaaaaa", "222222222222"}}), materialBody())
	writeMaterial(t, "broken.it.md", "# Dense\n\nNo header.", "Testo.\n")

	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json"))
	byPage := map[string]renderRow{}
	for _, row := range rows {
		byPage[row.Page] = row
	}
	if got := byPage["architecture/alpha"]; got.Status != renderMissing ||
		!strings.Contains(got.Detail, "no longer carries") {
		t.Errorf("an id the page lacks is missing, got %+v", got)
	}
	if got := byPage["architecture/gone"]; got.Status != renderOrphan {
		t.Errorf("a material whose page is gone is orphan, got %+v", got)
	}
	if got := byPage["software-architect/broken.it"]; got.Status != renderStale ||
		!strings.Contains(got.Detail, "not a material header") {
		t.Errorf("a header that cannot be read is stale, got %+v", got)
	}
}

// TestWikiMaterialCheckGates proves --check covers the material lane, not only the
// renders: a fresh material passes and a stale one fails the same gate.
func TestWikiMaterialCheckGates(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	writeMaterial(t, "alpha.it.md", materialHeaderLine("architecture/alpha",
		[][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}}), materialBody())

	execute(t, contextWikiRenderCmd, nil, "--check")

	writeMaterial(t, "alpha.it.md", materialHeaderLine("architecture/alpha",
		[][2]string{{"c-aaaaaaaa", "000000000000"}}), materialBody())
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiRenderCmd, nil, "--check"))
	})
}

// TestWikiRenderLaneStaysUnchanged proves the render row keeps its shape: no lane,
// its own status, and the two lanes share one report.
func TestWikiRenderLaneStaysUnchanged(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeMaterialPage(t, "architecture/alpha")
	writeRender(t, "architecture/alpha", renderHeader("architecture/alpha", "2026-10-20T10:00:00Z"))
	digests := liveDigests(t, "architecture/alpha")
	writeMaterial(t, "alpha.it.md", materialHeaderLine("architecture/alpha",
		[][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}}), materialBody())

	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json"))
	if len(rows) != 2 {
		t.Fatalf("want one render and one material, got %+v", rows)
	}
	var render, material *renderRow
	for i := range rows {
		if rows[i].Lane == materialLane {
			material = &rows[i]
			continue
		}
		render = &rows[i]
	}
	if render == nil || render.Status != renderFresh || render.Stamp != "2026-10-20T10:00:00Z" {
		t.Errorf("the render row lost its shape: %+v", render)
	}
	if material == nil || material.Status != renderFresh {
		t.Errorf("the material row is missing or not fresh: %+v", material)
	}
}

// proseGuide builds a guide body citing the given claim pairs and, optionally, a
// lesson section.
func proseGuide(pairs [][2]string, lesson string) string {
	body := "# Guida\n\n"
	for _, pair := range pairs {
		body += "Una frase.\n<!-- claim=architecture/alpha#" + pair[0] + " digest=" + pair[1] + " -->\n\n"
	}
	if lesson != "" {
		body += "L'esempio.\n<!-- lesson=architecture/alpha#" + lesson + " -->\n"
	}
	return body
}

// materialStatus returns the single material row of the report.
func materialStatus(t *testing.T) renderRow {
	t.Helper()
	for _, row := range renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json")) {
		if row.Lane == materialLane {
			return row
		}
	}
	t.Fatal("no material row in the report")
	return renderRow{}
}

// TestWikiMaterialProseClaimMarkers proves the prose is checked, not only the
// header: a moved digest, an id the page lacks and a marker for another concept are
// each refused with the marker named.
func TestWikiMaterialProseClaimMarkers(t *testing.T) {
	cases := []struct {
		name   string
		pairs  func(map[string]string) [][2]string
		status string
		detail string
	}{
		{"fresh", func(d map[string]string) [][2]string {
			return [][2]string{{"c-aaaaaaaa", d["c-aaaaaaaa"]}}
		}, renderFresh, ""},
		{"moved digest", func(d map[string]string) [][2]string {
			return [][2]string{{"c-aaaaaaaa", "000000000000"}}
		}, renderStale, "moved"},
		{"gone id", func(map[string]string) [][2]string {
			return [][2]string{{"c-zzzzzzzz", "111111111111"}}
		}, renderMissing, "not on the page"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupContextProject(t)
			stubWikiExit(t)
			writeMaterialPage(t, "architecture/alpha")
			digests := liveDigests(t, "architecture/alpha")
			header := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
			writeMaterial(t, "alpha.it.md", header, proseGuide(tc.pairs(digests), ""))
			got := materialStatus(t)
			if got.Status != tc.status {
				t.Errorf("status = %q, want %q (%+v)", got.Status, tc.status, got)
			}
			if tc.detail != "" && !strings.Contains(got.Detail, tc.detail) {
				t.Errorf("detail %q does not mention %q", got.Detail, tc.detail)
			}
		})
	}
}

// TestWikiMaterialProseForeignConcept proves a marker naming another page is refused:
// the binding is to the concept in the header, never to a neighbour.
func TestWikiMaterialProseForeignConcept(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	header := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
	body := "# Guida\n\nUna frase.\n<!-- claim=architecture/beta#c-aaaaaaaa digest=" + digests["c-aaaaaaaa"] + " -->\n"
	writeMaterial(t, "alpha.it.md", header, body)

	got := materialStatus(t)
	if got.Status != renderStale || !strings.Contains(got.Detail, "architecture/beta") {
		t.Errorf("a marker for another concept is stale and named, got %+v", got)
	}
}

// TestWikiMaterialProseLessonSection proves the lesson marker is resolved: a section
// the lesson carries passes, one it lacks is missing.
func TestWikiMaterialProseLessonSection(t *testing.T) {
	for _, tc := range []struct {
		section string
		status  string
	}{{"worked-example", renderFresh}, {"no-such-section", renderMissing}} {
		t.Run(tc.section, func(t *testing.T) {
			setupContextProject(t)
			stubWikiExit(t)
			writeMaterialPage(t, "architecture/alpha")
			writeCtxDoc(t, "context/learning/architecture/alpha.md",
				"---\nkind: lesson\nid: architecture/alpha\n---\n\n## Prerequisites\n\n## Worked example\n\n## Transfer\n")
			digests := liveDigests(t, "architecture/alpha")
			header := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
			writeMaterial(t, "alpha.it.md", header, proseGuide([][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}}, tc.section))

			got := materialStatus(t)
			if got.Status != tc.status {
				t.Errorf("lesson section %q: status = %q, want %q (%+v)", tc.section, got.Status, tc.status, got)
			}
		})
	}
}

// TestWikiMaterialProseSilenceIsNotFresh proves a guide that marks nothing carries no
// binding and says so, while the deck shape is exempt (its notes are prose).
func TestWikiMaterialProseSilenceIsNotFresh(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	header := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
	writeMaterial(t, "silent.it.md", header, "# Guida\n\nUna frase senza marker.\n")

	got := materialStatus(t)
	if got.Status != renderStale || !strings.Contains(got.Detail, "no claim, lesson or gloss marker") {
		t.Errorf("a guide with no marker is stale, got %+v", got)
	}

	writeCtxDoc(t, "context/renders/materials/software-architect/silent.it.slide.md", header+"\n\n# Slide\n\nUna slide.\n")
	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json"))
	for _, row := range rows {
		if strings.HasSuffix(row.Path, ".slide.md") && row.Status != renderFresh {
			t.Errorf("a deck is exempt from the prose markers, got %+v", row)
		}
	}
}

// writeObjectivePage writes a page carrying n learning objectives, the count the
// objective markers are compared with.
func writeObjectivePage(t *testing.T, id string, n int) {
	t.Helper()
	objectives := ""
	for i := 1; i <= n; i++ {
		objectives += "    - \"objective " + string(rune('0'+i)) + "\"\n"
	}
	body := strings.Replace(materialPage, "project: p\n", "project: p\nlearning:\n  objectives:\n"+objectives, 1)
	body = strings.Replace(body, "architecture/alpha", id, 1)
	writeCtxDoc(t, "context/wiki/"+id+".md", body)
}

// TestWikiMaterialProseObjectiveCount proves the section contract is countable: one
// marker per learning objective, none missing, none twice, none invented.
func TestWikiMaterialProseObjectiveCount(t *testing.T) {
	cases := []struct {
		name    string
		markers string
		status  string
		detail  string
	}{
		{"both", "<!-- objective=1 -->\n<!-- objective=2 -->\n", renderFresh, ""},
		{"missing second", "<!-- objective=1 -->\n", renderMissing, "objective 2 has no section"},
		{"duplicate", "<!-- objective=1 -->\n<!-- objective=1 -->\n<!-- objective=2 -->\n", renderStale, "counted 2 times"},
		{"invented", "<!-- objective=1 -->\n<!-- objective=2 -->\n<!-- objective=7 -->\n", renderStale, "not one of the page's 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupContextProject(t)
			stubWikiExit(t)
			writeObjectivePage(t, "architecture/alpha", 2)
			digests := liveDigests(t, "architecture/alpha")
			header := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
			body := proseGuide([][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}}, "") + "\n" + tc.markers
			writeMaterial(t, "alpha.it.md", header, body)

			got := materialStatus(t)
			if got.Status != tc.status {
				t.Errorf("status = %q, want %q (%+v)", got.Status, tc.status, got)
			}
			if tc.detail != "" && !strings.Contains(got.Detail, tc.detail) {
				t.Errorf("detail %q does not mention %q", got.Detail, tc.detail)
			}
		})
	}
}
