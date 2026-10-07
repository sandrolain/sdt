package cmd

import (
	"fmt"
	"os"

	"github.com/sandrolain/sdt/internal/semantic"
)

// Semantic lint advisory: an additive SUGGESTION that surfaces documents the
// lexical (Jaccard) pass misses. It reads the persisted snapshot only (no
// encoder, no model load) and degrades to nothing when the snapshot is absent.
// A pair already flagged by the lexical pass is not reported again.

// ctxSemanticDupThreshold is the document cosine above which two notes are
// reported as semantically near-duplicate. It is higher than the lexical
// Jaccard threshold because cosine over averaged section vectors is coarser.
const ctxSemanticDupThreshold = 0.90

// ctxSemanticOverlapThreshold is the document cosine above which two
// same-objective analyses are reported as overlapping.
const ctxSemanticOverlapThreshold = 0.88

// lintSemanticDuplicates reports note pairs whose semantic cosine crosses
// ctxSemanticDupThreshold but whose lexical Jaccard is below the dedup
// threshold (the pairs the lexical pass does not already flag).
func lintSemanticDuplicates(files []string, snap *semantic.Snapshot) []ctxLintIssue {
	if snap == nil || snap.Empty() || len(files) < 2 {
		return nil
	}
	tokens := map[string]map[string]struct{}{}
	for _, f := range files {
		if data, err := os.ReadFile(f); err == nil { //#nosec G304 -- fixed repo path
			tokens[f] = tokenSet(normalizedBody(noteBody(string(data))))
		}
	}
	var issues []ctxLintIssue
	for _, p := range snap.SimilarPairs(files, ctxSemanticDupThreshold) {
		if ja, ok := tokens[p.A]; ok {
			if jb, ok2 := tokens[p.B]; ok2 && jaccard(ja, jb) >= ctxDupJaccardThreshold {
				continue // already flagged by the lexical pass
			}
		}
		issues = append(issues, ctxLintIssue{
			Path:     p.B,
			Priority: ctxLintSuggestion,
			Message:  fmt.Sprintf("semantically near-duplicate of %s (cosine %.2f); consolidate or link (dedup-before-write)", p.A, p.Score),
		})
	}
	return issues
}

// lintSemanticOverlaps reports same-objective analysis pairs whose semantic
// cosine crosses ctxSemanticOverlapThreshold but whose title/summary Jaccard is
// below the lexical overlap threshold (the pairs the lexical pass misses).
func lintSemanticOverlaps(files []string, snap *semantic.Snapshot) []ctxLintIssue {
	if snap == nil || snap.Empty() || len(files) < 2 {
		return nil
	}
	byObjective := map[string][]string{}
	sig := map[string]map[string]struct{}{}
	for _, f := range files {
		data, err := os.ReadFile(f) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		obj := parseFrontmatterField(string(data), "objective")
		if obj == "" {
			continue
		}
		byObjective[obj] = append(byObjective[obj], f)
		sig[f] = tokenSet(analysisSignature(f, string(data)))
	}
	var issues []ctxLintIssue
	for _, group := range byObjective {
		for _, p := range snap.SimilarPairs(group, ctxSemanticOverlapThreshold) {
			if sa, ok := sig[p.A]; ok {
				if sb, ok2 := sig[p.B]; ok2 && jaccard(sa, sb) >= ctxAnalysisOverlapThreshold {
					continue // already flagged by the lexical pass
				}
			}
			issues = append(issues, ctxLintIssue{
				Path:     p.B,
				Priority: ctxLintSuggestion,
				Message:  fmt.Sprintf("semantically overlaps analysis %s (cosine %.2f); link it (`links`/`supersedes`) or state why they differ", p.A, p.Score),
			})
		}
	}
	return issues
}
