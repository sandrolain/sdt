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
func writeMaterial(t *testing.T, rel, header string) string {
	t.Helper()
	path := "context/renders/materials/software-architect/" + rel
	writeCtxDoc(t, path, header+"\n\n# Guida\n\nTesto.\n")
	return path
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
	writeMaterial(t, "architecture-measurement.it.md", fresh)
	stale := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]}, {"c-bbbbbbbb", "000000000000"},
	})
	writeMaterial(t, "alpha.it.md", stale)

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
		[][2]string{{"c-zzzzzzzz", "111111111111"}}))
	writeMaterial(t, "gone.it.md", materialHeaderLine("architecture/gone",
		[][2]string{{"c-aaaaaaaa", "222222222222"}}))
	writeMaterial(t, "broken.it.md", "# Dense\n\nNo header.")

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
		[][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}}))

	execute(t, contextWikiRenderCmd, nil, "--check")

	writeMaterial(t, "alpha.it.md", materialHeaderLine("architecture/alpha",
		[][2]string{{"c-aaaaaaaa", "000000000000"}}))
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
		[][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}}))

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
