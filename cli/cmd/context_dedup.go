package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// Dedup-before-write lint for context notes. Advisory only: it emits
// SUGGESTION issues, never merges documents and never fails a lint run.

// ctxDupJaccardThreshold is the word-set Jaccard similarity above which two
// distinct notes are reported as near-duplicates.
const ctxDupJaccardThreshold = 0.80

// ctxDupBucketWidth buckets notes by normalized body length (bytes) so only
// notes of comparable size are ever compared pairwise (O(n·k), not O(n²)).
const ctxDupBucketWidth = 256

// normalizedBody canonicalizes text for comparison: ASCII lowercased and all
// whitespace collapsed to single spaces (empty fields dropped).
func normalizedBody(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// noteBody returns the markdown body of a context document: everything after
// the closing YAML frontmatter, or the whole content when no frontmatter.
func noteBody(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	if i := strings.Index(content, "\n---\n"); i >= 0 {
		return content[i+len("\n---\n"):]
	}
	return content
}

// tokenSet builds the set of words in s for Jaccard comparison.
func tokenSet(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, tok := range strings.Fields(s) {
		set[tok] = struct{}{}
	}
	return set
}

// jaccard returns the Jaccard coefficient (|A∩B| / |A∪B|) of two token sets.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// contentFingerprint is a stable 16-hex-char SHA-256 digest of the normalized
// note body, used to group exact duplicates.
func contentFingerprint(s string) string {
	sum := sha256.Sum256([]byte(normalizedBody(s)))
	return hex.EncodeToString(sum[:8])
}

// lintDuplicateNotes scans context note files for exact and near-duplicate
// bodies. Generated map documents (`.map.md`) are skipped: they are derived
// artifacts, not authored notes. Exact duplicates are grouped by content
// fingerprint (one issue per duplicate file); near-duplicates are detected
// pairwise within adjacent body-length buckets via word-set Jaccard.
func lintDuplicateNotes(files []string) []ctxLintIssue {
	if len(files) < 2 {
		return nil
	}
	type noteCmp struct {
		path  string
		body  string
		token map[string]struct{}
	}
	var notes []noteCmp
	seen := make(map[string]string)
	var issues []ctxLintIssue
	for _, f := range files {
		if contextwiki.IsMapDoc(f) {
			continue
		}
		data, err := os.ReadFile(f) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		body := normalizedBody(noteBody(string(data)))
		if body == "" {
			continue
		}
		if first, ok := seen[contentFingerprint(body)]; ok {
			issues = append(issues, ctxLintIssue{
				Path:     f,
				Priority: ctxLintSuggestion,
				Message:  fmt.Sprintf("exact duplicate of %s; consolidate or link (dedup-before-write)", first),
			})
			continue
		}
		seen[contentFingerprint(body)] = f
		notes = append(notes, noteCmp{path: f, body: body, token: tokenSet(body)})
	}
	if len(notes) < 2 {
		return issues
	}
	buckets := make(map[int][]int)
	for i, n := range notes {
		b := len(n.body) / ctxDupBucketWidth
		buckets[b] = append(buckets[b], i)
	}
	var keys []int
	for b := range buckets {
		keys = append(keys, b)
	}
	sort.Ints(keys)
	for k, b := range keys {
		idxs := append([]int{}, buckets[b]...)
		if k+1 < len(keys) {
			idxs = append(idxs, buckets[keys[k+1]]...)
		}
		for i := 0; i < len(idxs); i++ {
			for j := i + 1; j < len(idxs); j++ {
				a, b := notes[idxs[i]], notes[idxs[j]]
				if jc := jaccard(a.token, b.token); jc >= ctxDupJaccardThreshold {
					issues = append(issues, ctxLintIssue{
						Path:     b.path,
						Priority: ctxLintSuggestion,
						Message:  fmt.Sprintf("near-duplicate of %s (Jaccard %.2f); consolidate or link (dedup-before-write)", a.path, jc),
					})
				}
			}
		}
	}
	return issues
}
