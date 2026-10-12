package search

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/ctxquery"
)

// handleShape classifies a query as a document handle — the identifier a
// document is addressed by — rather than as free text.
type handleShape int

const (
	// handleNone is any query that is not handle-shaped: the lexical ranking is
	// the only answer.
	handleNone handleShape = iota
	// handleDateTime is the `YYYYMMDD-HHMMSS` prefix every dated work file
	// carries, the shape a user pastes out of a filename or an index row.
	handleDateTime
	// handleUID is a canonical UUID, the value of the `uid` frontmatter key.
	handleUID
)

var (
	dateTimeHandleRe = regexp.MustCompile(`^\d{8}-\d{6}$`)
	uidHandleRe      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// handleScore is the relevance of a handle-resolved hit: an identifier match is
// the strongest answer a query can have, so it outranks every textual score
// while staying a finite number the JSON output can carry.
const handleScore = 1000

// classifyHandle reports which handle shape q is, or handleNone.
func classifyHandle(q string) handleShape {
	switch {
	case dateTimeHandleRe.MatchString(q):
		return handleDateTime
	case uidHandleRe.MatchString(q):
		return handleUID
	default:
		return handleNone
	}
}

// resolveHandle resolves a handle-shaped query against the entry set: the raw
// filename — which carries the date-time prefix `doc.Name` strips — and the
// `uid` frontmatter value. Hits come back in path order, so a handle matching
// more than one document stays deterministic, and the shared filter applies to
// them exactly as it does to a lexical hit.
//
// It returns nil when the query is not handle-shaped or nothing matches, so a
// false positive degrades to the lexical ranking instead of answering nothing.
// ResolveHandles returns the documents a handle-shaped query addresses, or nil.
// It is exported so the hybrid path can re-assert the resolution after fusion:
// RRF is rank-based and would otherwise let a semantic-only hit overtake the
// document the user named.
func (ix *Index) ResolveHandles(q string, filter *ctxquery.Filter) []doc {
	return ix.resolveHandle(q, filter)
}

func (ix *Index) resolveHandle(q string, filter *ctxquery.Filter) []doc {
	shape := classifyHandle(q)
	if shape == handleNone {
		return nil
	}
	lower := strings.ToLower(q)
	hits := make([]doc, 0, 1)
	for _, d := range ix.registry {
		if filter != nil && !filter.Match(d.facets()) {
			continue
		}
		if handleMatches(d, shape, lower) {
			hits = append(hits, d)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Path < hits[j].Path })
	return hits
}

// handleMatches reports whether the handle addresses d. A date-time handle
// matches the filename it prefixes; a uid matches the `uid` frontmatter exactly
// (case-insensitive) and also a file named after its own id.
func handleMatches(d doc, shape handleShape, lower string) bool {
	if strings.Contains(strings.ToLower(filepath.Base(d.Path)), lower) {
		return true
	}
	if shape == handleUID {
		for _, v := range d.Values["uid"] {
			if strings.EqualFold(strings.TrimSpace(v), lower) {
				return true
			}
		}
	}
	return false
}

// prependHandles ranks the handle-resolved documents above the lexical hits,
// drops their lexical duplicates, keeps the lexical order for the rest and
// returns at most max results. Total stays the honest match count: the resolved
// documents plus whatever the lexical branch counted.
func (ix *Index) prependHandles(handles []doc, lex Results, q HybridQuery) Results {
	seen := make(map[string]struct{}, len(handles))
	out := make([]Result, 0, min(q.Max, len(handles)+len(lex.Results)))
	for _, d := range handles {
		if _, dup := seen[d.Path]; dup {
			continue
		}
		seen[d.Path] = struct{}{}
		out = append(out, ix.resultForDoc(d, handleScore, q.Q))
	}
	for _, r := range lex.Results {
		if len(out) >= q.Max {
			break
		}
		if _, dup := seen[r.Path]; dup {
			continue
		}
		seen[r.Path] = struct{}{}
		out = append(out, r)
	}
	total := int64(len(handles)) + lex.Total
	return Results{Results: out, Total: total, Partial: lex.Partial}
}
