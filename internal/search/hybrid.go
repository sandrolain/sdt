package search

import "sort"

// Reciprocal Rank Fusion (RRF) over the lexical and semantic rankings, keyed by
// the shared section id (`path#anchor`) or the document path when a hit is
// document-level. RRF is rank-based, so it needs no score normalization between
// the two engines.

// RRFK is the RRF smoothing constant (Cormack et al. use 60).
const RRFK = 60

// ranked is one item in a ranking: an id and its rank (1-based).
type ranked struct {
	ID   string
	Rank int
}

// rrfScore accumulates Reciprocal Rank Fusion scores for a set of rankings.
// The returned map is id -> fused score; ties are broken by the caller.
func rrfScore(rankings [][]ranked) map[string]float64 {
	scores := map[string]float64{}
	for _, ranking := range rankings {
		for _, r := range ranking {
			if r.ID == "" {
				continue
			}
			scores[r.ID] += 1.0 / float64(RRFK+r.Rank)
		}
	}
	return scores
}

// fuseHybrid merges the lexical hits and the semantic section hits into one
// ranked list. Lexical hits are document-level (keyed by path) and semantic
// hits are section-level (keyed by `path#anchor`); a semantic hit whose section
// belongs to a lexically-matched document contributes to that document, so the
// two signals reinforce rather than duplicate.
func fuseHybrid(lexical []Result, semanticIDs []string) []Result {
	if len(semanticIDs) == 0 {
		return lexical
	}
	lexRanking := make([]ranked, 0, len(lexical))
	byPath := make(map[string]Result, len(lexical))
	for i, hit := range lexical {
		lexRanking = append(lexRanking, ranked{ID: hit.Path, Rank: i + 1})
		byPath[hit.Path] = hit
	}
	semRanking := make([]ranked, 0, len(semanticIDs))
	for i, id := range semanticIDs {
		path, _ := splitSectionID(id)
		semRanking = append(semRanking, ranked{ID: path, Rank: i + 1})
	}
	scores := rrfScore([][]ranked{lexRanking, semRanking})

	out := make([]Result, 0, len(scores))
	for path, score := range scores {
		hit, ok := byPath[path]
		if !ok {
			// Semantic-only document: no lexical hit, so build the display
			// fields from the section metadata.
			hit = Result{Path: path, Section: anchorFor(semanticIDs, path)}
		}
		hit.Score = score
		out = append(out, hit)
	}
	sortResultsByScore(out)
	return out
}

// anchorFor returns the first semantic anchor recorded for a path, if any.
func anchorFor(semanticIDs []string, path string) string {
	for _, id := range semanticIDs {
		if p, a := splitSectionID(id); p == path {
			return a
		}
	}
	return ""
}

// splitSectionID splits `path#anchor` into path and anchor.
func splitSectionID(id string) (path, anchor string) {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '#' {
			return id[:i], id[i+1:]
		}
	}
	return id, ""
}

// sortResultsByScore orders results by descending fused score, then by path for
// a stable, deterministic tie-break.
func sortResultsByScore(hits []Result) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
}
