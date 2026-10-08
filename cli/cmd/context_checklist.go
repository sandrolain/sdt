package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Checklist items are the shared `- [ ] text` lines used by task files (the
// execution unit) and by plan/question bodies. Each item carries a stable
// per-document identifier — a trailing HTML-comment anchor `<!-- c<N> -->`,
// invisible through the viewer's marked+DOMPurify pipeline — so the CLI can
// address an item without depending on its position.
//
// An item may span several lines: the checklist line plus its indented
// continuation lines. The anchor is a property of the item — hand-authored
// items often place it at the end of the last continuation line — so it is read
// on any of the item's lines and, on write, normalized onto the checklist line.

// ctxChecklistAnchorRegexp captures the item text preceding a trailing anchor.
var ctxChecklistAnchorRegexp = regexp.MustCompile(`^(.*?)\s*<!--\s*c([0-9]+)\s*-->\s*$`)

// ctxNestedChecklistRegexp matches an indented nested checklist line, which is
// not an item of the current document and ends the enclosing item's span.
var ctxNestedChecklistRegexp = regexp.MustCompile(`^[-*+] \[[ x~!]\]`)

// checklistItem is one checklist line with its 1-based position among the
// document's checklist items, marker, status and optional id anchor.
type checklistItem struct {
	Line   int    `json:"line" yaml:"line"`
	Marker string `json:"marker" yaml:"marker"`
	Status string `json:"status" yaml:"status"`
	Text   string `json:"text" yaml:"text"`
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
}

// checklistEntry is one checklist item as a span of document lines: the
// checklist line at First and its continuation lines up to Last (inclusive).
// ID is the item's anchor, found on any of its lines (checklist line preferred).
type checklistEntry struct {
	First  int
	Last   int
	Marker string
	Body   string
	ID     string
}

// checklistMarker maps an item status (or the `block` verb) to its checkbox
// marker character.
func checklistMarker(status string) string {
	switch status {
	case taskStatusDone:
		return "x"
	case taskStatusWip:
		return "~"
	case taskStatusBlocked, taskStatusBlock:
		return "!"
	default:
		return " "
	}
}

// checklistStatus maps a checkbox marker to its item status.
func checklistStatus(marker string) string {
	switch marker {
	case "x":
		return taskStatusDone
	case "~":
		return taskStatusWip
	case "!":
		return taskStatusBlocked
	default:
		return taskStatusTodo
	}
}

// parseChecklistID normalizes a user-supplied item id ("c3", "3", "C3") to its
// number.
func parseChecklistID(arg string) (int, bool) {
	s := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(arg)), "c")
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// splitChecklistAnchor splits an item's raw text into body and id anchor.
func splitChecklistAnchor(text string) (body, id string) {
	if m := ctxChecklistAnchorRegexp.FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1]), "c" + m[2]
	}
	return strings.TrimSpace(text), ""
}

// parseChecklistEntries returns every checklist item of content as a line span.
// Lines inside fenced code blocks are ignored, so a `- [ ]` sample in a snippet
// is never treated as an item.
func parseChecklistEntries(lines []string) []checklistEntry {
	var entries []checklistEntry
	fence := ""
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = t[:3]
			continue
		}
		if ctxTaskLineRegexp.FindStringSubmatch(lines[i]) == nil {
			continue
		}
		last := i
		for j := i + 1; j < len(lines) && checklistContinuation(lines[j]); j++ {
			last = j
		}
		entries = append(entries, newChecklistEntry(lines, i, last))
		i = last
	}
	return entries
}

// checklistContinuation reports whether line belongs to the preceding checklist
// item: indented, non-blank, not a heading and not a nested checklist.
func checklistContinuation(line string) bool {
	if line == "" || (line[0] != ' ' && line[0] != '\t') {
		return false
	}
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") || ctxNestedChecklistRegexp.MatchString(t) {
		return false
	}
	return true
}

// checklistLineText returns the addressable text of a line: the body after the
// checkbox for a checklist line, or the trimmed line for a continuation line.
func checklistLineText(line string) string {
	if m := ctxTaskLineRegexp.FindStringSubmatch(line); m != nil {
		return m[2]
	}
	return strings.TrimSpace(line)
}

// newChecklistEntry builds the entry for the span lines[first:last+1], resolving
// the item's anchor across the span (checklist line preferred).
func newChecklistEntry(lines []string, first, last int) checklistEntry {
	body, id := splitChecklistAnchor(checklistLineText(lines[first]))
	m := ctxTaskLineRegexp.FindStringSubmatch(lines[first])
	e := checklistEntry{First: first, Last: last, Marker: m[1], Body: body, ID: id}
	if e.ID == "" {
		for i := last; i > first; i-- {
			if _, sid := splitChecklistAnchor(checklistLineText(lines[i])); sid != "" {
				e.ID = sid
				break
			}
		}
	}
	return e
}

// parseChecklistItems returns every checklist item of content in body order.
func parseChecklistItems(content string) []checklistItem {
	entries := parseChecklistEntries(strings.Split(content, "\n"))
	items := make([]checklistItem, 0, len(entries))
	for i, e := range entries {
		items = append(items, checklistItem{
			Line:   i + 1,
			Marker: e.Marker,
			Status: checklistStatus(e.Marker),
			Text:   e.Body,
			ID:     e.ID,
		})
	}
	return items
}

// stampChecklistIDs assigns a fresh `<!-- c<N> -->` anchor to every checklist
// item that lacks one. N is monotonic within the document (max existing + 1),
// computed over items so an anchor on a continuation line counts; an item that
// already carries an anchor on any line is preserved, so no item ever ends up
// with two anchors. It reports whether content changed.
func stampChecklistIDs(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	entries := parseChecklistEntries(lines)
	max := 0
	for _, e := range entries {
		if n, ok := parseChecklistID(e.ID); ok && n > max {
			max = n
		}
	}
	changed := false
	for _, e := range entries {
		if e.ID != "" {
			continue
		}
		max++
		lines[e.First] = strings.TrimRight(lines[e.First], " \t") + fmt.Sprintf(" <!-- c%d -->", max)
		changed = true
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

// updateChecklistItem rewrites the marker of the item addressed by idArg — the
// anchor id ("c3"/"3") first, the positional ordinal as fallback — preserving
// the item text. Un-anchored items are lazily stamped first, and the item's
// anchor is normalized onto its checklist line (a stray anchor on a continuation
// line is dropped). An id matching more than one item is refused as ambiguous.
func updateChecklistItem(content, idArg, status, reason string) (string, error) {
	stamped, _ := stampChecklistIDs(content)
	lines := strings.Split(stamped, "\n")
	e, err := resolveChecklistEntry(lines, idArg)
	if err != nil {
		return "", err
	}
	body, _ := splitChecklistAnchor(checklistLineText(lines[e.First]))
	updated := fmt.Sprintf("- [%s] %s", checklistMarker(status), body)
	if status == taskStatusBlock && reason != "" {
		updated += fmt.Sprintf(" (blocked: %s)", reason)
	}
	if e.ID != "" {
		updated += " <!-- " + e.ID + " -->"
	}
	lines[e.First] = updated
	for i := e.First + 1; i <= e.Last; i++ {
		if b, sid := splitChecklistAnchor(checklistLineText(lines[i])); sid != "" {
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
			lines[i] = strings.TrimRight(indent+b, " \t")
		}
	}
	return strings.Join(lines, "\n"), nil
}

// updateChecklistWaiver marks an item as waived: it keeps the item unfinished
// (marker `~`) and records a structured `(waived: <reason>)` annotation, so a
// verification that could not run is never a false pass (F16). An id matching
// more than one item is refused as ambiguous.

func updateChecklistWaiver(content, idArg, reason string) (string, error) {
	stamped, _ := stampChecklistIDs(content)
	lines := strings.Split(stamped, "\n")
	e, err := resolveChecklistEntry(lines, idArg)
	if err != nil {
		return "", err
	}
	body, _ := splitChecklistAnchor(checklistLineText(lines[e.First]))
	waived := fmt.Sprintf("- [~] %s (waived: %s)", body, strings.TrimSpace(reason))
	if e.ID != "" {
		waived += " <!-- " + e.ID + " -->"
	}
	lines[e.First] = waived
	for i := e.First + 1; i <= e.Last; i++ {
		if b, sid := splitChecklistAnchor(checklistLineText(lines[i])); sid != "" {
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
			lines[i] = strings.TrimRight(indent+b, " \t")
		}
	}
	return strings.Join(lines, "\n"), nil
}

// repairChecklistIDs normalizes the checklist ids of one document: it moves an
// item's anchor onto its checklist line (dropping a stray anchor from a
// continuation line), then renumbers later duplicates of an id to the next free
// number so every id addresses exactly one item. It reports whether content
// changed and is idempotent. Only repair lines are rewritten; a clean line keeps
// its exact spacing.
func repairChecklistIDs(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	entries := parseChecklistEntries(lines)
	if len(entries) == 0 {
		return content, false
	}
	changed := false
	// Adopt an anchor found only on a continuation line; drop stray continuation
	// anchors when the checklist line already carries the item id.
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		if _, cid := splitChecklistAnchor(checklistLineText(lines[e.First])); cid != e.ID {
			body, _ := splitChecklistAnchor(checklistLineText(lines[e.First]))
			lines[e.First] = fmt.Sprintf("- [%s] %s <!-- %s -->", e.Marker, body, e.ID)
			changed = true
		}
		for j := e.First + 1; j <= e.Last; j++ {
			if b, sid := splitChecklistAnchor(checklistLineText(lines[j])); sid != "" {
				indent := lines[j][:len(lines[j])-len(strings.TrimLeft(lines[j], " \t"))]
				lines[j] = strings.TrimRight(indent+b, " \t")
				changed = true
			}
		}
	}
	// Renumber later duplicates globally (first occurrence wins).
	entries = parseChecklistEntries(lines)
	next := 0
	for _, e := range entries {
		if n, ok := parseChecklistID(e.ID); ok && n > next {
			next = n
		}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		if !seen[e.ID] {
			seen[e.ID] = true
			continue
		}
		next++
		newID := "c" + strconv.Itoa(next)
		body, _ := splitChecklistAnchor(checklistLineText(lines[e.First]))
		lines[e.First] = fmt.Sprintf("- [%s] %s <!-- %s -->", e.Marker, body, newID)
		seen[newID] = true
		changed = true
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

// resolveChecklistEntry returns the item addressed by idArg: the anchor id
// first, the positional ordinal as fallback. A repeated anchor id is refused as
// ambiguous rather than silently resolved to its first match.
func resolveChecklistEntry(lines []string, idArg string) (checklistEntry, error) {
	entries := parseChecklistEntries(lines)
	if n, ok := parseChecklistID(idArg); ok {
		anchor := "c" + strconv.Itoa(n)
		var hits []checklistEntry
		for _, e := range entries {
			if e.ID == anchor {
				hits = append(hits, e)
			}
		}
		if len(hits) > 1 {
			return checklistEntry{}, fmt.Errorf("task id %q is ambiguous: %d items carry anchor %s; run `sdt context checklist backfill` to renumber duplicates", idArg, len(hits), anchor)
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
	}
	n, err := strconv.Atoi(strings.TrimSpace(idArg))
	if err != nil || n < 1 || n > len(entries) {
		return checklistEntry{}, fmt.Errorf("task id %q not found", idArg)
	}
	return entries[n-1], nil
}

// resolveChecklistItem returns the item addressed by idArg — the anchor id
// first, the positional ordinal as fallback. An ambiguous anchor id is not
// resolved (ok false).
func resolveChecklistItem(content, idArg string) (checklistItem, bool) {
	items := parseChecklistItems(content)
	if n, ok := parseChecklistID(idArg); ok {
		anchor := "c" + strconv.Itoa(n)
		var hits []checklistItem
		for _, it := range items {
			if it.ID == anchor {
				hits = append(hits, it)
			}
		}
		if len(hits) == 1 {
			return hits[0], true
		}
		if len(hits) > 1 {
			return checklistItem{}, false
		}
	}
	if n, err := strconv.Atoi(strings.TrimSpace(idArg)); err == nil && n >= 1 {
		for _, it := range items {
			if it.Line == n {
				return it, true
			}
		}
	}
	return checklistItem{}, false
}
