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

// ctxChecklistAnchorRegexp captures the item text preceding a trailing anchor.
var ctxChecklistAnchorRegexp = regexp.MustCompile(`^(.*?)\s*<!--\s*c([0-9]+)\s*-->\s*$`)

// checklistItem is one checklist line with its 1-based position among the
// document's checklist items, marker, status and optional id anchor.
type checklistItem struct {
	Line   int    `json:"line" yaml:"line"`
	Marker string `json:"marker" yaml:"marker"`
	Status string `json:"status" yaml:"status"`
	Text   string `json:"text" yaml:"text"`
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
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

// parseChecklistItems returns every checklist line of content in body order.
func parseChecklistItems(content string) []checklistItem {
	var items []checklistItem
	ord := 0
	for _, line := range strings.Split(content, "\n") {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ord++
		body, id := splitChecklistAnchor(m[2])
		items = append(items, checklistItem{
			Line:   ord,
			Marker: m[1],
			Status: checklistStatus(m[1]),
			Text:   body,
			ID:     id,
		})
	}
	return items
}

// stampChecklistIDs assigns a fresh `<!-- c<N> -->` anchor to every checklist
// item that lacks one. N is monotonic within the document (max existing + 1),
// so an id is never re-bound to a different item. It reports whether content
// changed.
func stampChecklistIDs(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	max := 0
	for _, line := range lines {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if _, id := splitChecklistAnchor(m[2]); id != "" {
			if n, ok := parseChecklistID(id); ok && n > max {
				max = n
			}
		}
	}
	changed := false
	for i, line := range lines {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		body, id := splitChecklistAnchor(m[2])
		if id != "" {
			continue
		}
		max++
		lines[i] = fmt.Sprintf("- [%s] %s <!-- c%d -->", m[1], body, max)
		changed = true
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

// updateChecklistItem rewrites the marker of the item addressed by idArg — the
// anchor id ("c3"/"3") first, the positional ordinal as fallback — preserving
// the item text and its anchor. Un-anchored items are lazily stamped first, so
// any CLI write migrates the items it touches.
func updateChecklistItem(content, idArg, status, reason string) (string, error) {
	stamped, _ := stampChecklistIDs(content)
	lines := strings.Split(stamped, "\n")
	target := checklistLineFor(lines, idArg)
	if target < 0 {
		return "", fmt.Errorf("task id %q not found", idArg)
	}
	m := ctxTaskLineRegexp.FindStringSubmatch(lines[target])
	body, id := splitChecklistAnchor(m[2])
	updated := fmt.Sprintf("- [%s] %s", checklistMarker(status), body)
	if status == taskStatusBlock && reason != "" {
		updated += fmt.Sprintf(" (blocked: %s)", reason)
	}
	if id != "" {
		updated += " <!-- " + id + " -->"
	}
	lines[target] = updated
	return strings.Join(lines, "\n"), nil
}

// resolveChecklistItem returns the item addressed by idArg — the anchor id
// first, the positional ordinal as fallback — mirroring checklistLineFor.
func resolveChecklistItem(content, idArg string) (checklistItem, bool) {
	items := parseChecklistItems(content)
	if n, ok := parseChecklistID(idArg); ok {
		anchor := "c" + strconv.Itoa(n)
		for _, it := range items {
			if it.ID == anchor {
				return it, true
			}
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

// checklistLineFor resolves idArg to a line index: the anchor id when present,
// otherwise the 1-based ordinal among checklist items, or -1.
func checklistLineFor(lines []string, idArg string) int {
	anchor := ""
	if n, ok := parseChecklistID(idArg); ok {
		anchor = "c" + strconv.Itoa(n)
	}
	if anchor != "" {
		for i, line := range lines {
			m := ctxTaskLineRegexp.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if _, id := splitChecklistAnchor(m[2]); id == anchor {
				return i
			}
		}
	}
	n, err := strconv.Atoi(strings.TrimSpace(idArg))
	if err != nil || n < 1 {
		return -1
	}
	ord := 0
	for i, line := range lines {
		if ctxTaskLineRegexp.FindStringSubmatch(line) == nil {
			continue
		}
		ord++
		if ord == n {
			return i
		}
	}
	return -1
}
