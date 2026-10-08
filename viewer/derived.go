package main

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/mdindex"
)

// Derived payloads (O1): /api/tree and /api/backlinks are built from the shared
// startup manifest (mdindex), which already indexes markdown plus the viewable
// .canvas/.mmd resources, instead of a per-request corpus walk. The ctxrel
// lifecycle edges are resolved once per rebuild, not per request.

// treeEntries returns the current manifest-derived tree entries (read-only).
func (s *server) treeEntries() []treeEntry {
	s.treeMu.RLock()
	defer s.treeMu.RUnlock()
	return s.tree
}

// rebuildDerived rebuilds the tree entries and the backlink index from a
// manifest and stores them for the request handlers.
func (s *server) rebuildDerived(m *mdindex.Manifest) {
	edges, err := ctxrel.Load(s.corpus)
	if err != nil {
		edges = &ctxrel.Edges{}
	}
	s.treeMu.Lock()
	s.tree = buildTree(m, edges)
	s.backlinks = buildBacklinkIndexFromManifest(m, edges)
	s.treeMu.Unlock()
}

// buildTree maps manifest entries to tree entries (payload shape unchanged) and
// stamps the resolved lifecycle edges.
func buildTree(m *mdindex.Manifest, edges *ctxrel.Edges) []treeEntry {
	if m == nil {
		return []treeEntry{}
	}
	entries := make([]treeEntry, 0, len(m.Entries))
	for _, e := range m.Entries {
		entries = append(entries, treeEntryFromEntry(e))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	applyResolvedEdges(entries, edges)
	return entries
}

func treeEntryFromEntry(e *mdindex.Entry) treeEntry {
	created := e.Created
	if created == "" {
		created = firstValue(e.Values, "created_at")
	}
	modified := e.Updated
	if modified == "" && e.ModTimeNS != 0 {
		modified = time.Unix(0, e.ModTimeNS).UTC().Format(time.RFC3339)
	}
	t := treeEntry{
		Path:       e.ID,
		Kind:       e.Kind,
		Title:      e.Title,
		Summary:    e.Summary,
		Objective:  e.Objective,
		Status:     e.Status,
		Categories: e.Categories,
		Sources:    e.Values["sources"],
		Created:    created,
		Modified:   modified,
		Image:      firstValue(e.Values, "image"),
	}
	switch e.Kind {
	case "canvas":
		t.Canvas = true
	case "mermaid":
		t.Mermaid = true
	}
	if contextwiki.IsMapDoc(e.ID) {
		t.IsMap = true
		t.MapID = contextwiki.DocID(e.ID)
	}
	return t
}

// buildBacklinkIndexFromManifest records every outbound reference of a markdown
// entry as an inbound edge on its target (sources/links + the ctxrel parent),
// mirroring the former corpus walk without re-reading the files.
func buildBacklinkIndexFromManifest(m *mdindex.Manifest, edges *ctxrel.Edges) *backlinkIndex {
	index := &backlinkIndex{byTarget: map[string][]backlinkDoc{}}
	if m == nil {
		return index
	}
	for _, e := range m.Entries {
		if filepath.Ext(e.ID) != markdownExt {
			continue
		}
		referrer := backlinkDoc{Path: e.ID, Title: e.Title, Kind: e.Kind, Summary: e.Summary, Modified: e.Updated}
		if referrer.Modified == "" && e.ModTimeNS != 0 {
			referrer.Modified = time.Unix(0, e.ModTimeNS).UTC().Format(time.RFC3339)
		}
		seen := map[string]bool{}
		record := func(ref, via string) {
			target := normalizeBacklinkRef(ref)
			if target == "" || target == e.ID || seen[target] {
				return
			}
			seen[target] = true
			doc := referrer
			doc.Via = via
			index.byTarget[target] = append(index.byTarget[target], doc)
		}
		for _, ref := range e.Values["sources"] {
			record(ref, viaSources)
		}
		for _, ref := range e.Values["links"] {
			record(ref, viaLinks)
		}
		switch e.Kind {
		case "plan", "tasks":
			if parent := edges.ParentOf(e.ID); parent != "" {
				record(parent, viaRelations)
			}
		}
	}
	index.sortAll()
	return index
}

func firstValue(values map[string][]string, key string) string {
	if vs := values[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}
