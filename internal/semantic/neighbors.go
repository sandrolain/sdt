package semantic

import (
	"math"
	"sort"
	"strings"
)

// Neighbor is one document's cosine similarity to a seed document.
type Neighbor struct {
	Path  string
	Score float64
}

// SectionPath returns the document path of a section id ("path#anchor").
func SectionPath(id string) string {
	if i := strings.IndexByte(id, '#'); i >= 0 {
		return id[:i]
	}
	return id
}

// DocumentVectors aggregates the snapshot's section vectors per document path
// (the mean of its section vectors), for document-level similarity. It needs no
// encoder: the comparison uses the persisted vectors only.
func (s *Snapshot) DocumentVectors() map[string][]float32 {
	sums := make(map[string][]float32)
	counts := make(map[string]int)
	for id, v := range s.SectionVectors {
		p := SectionPath(id)
		sum := sums[p]
		if sum == nil {
			sum = make([]float32, len(v))
			sums[p] = sum
		}
		if len(sum) != len(v) {
			continue // dimension mismatch: skip the outlier
		}
		for i := range v {
			sum[i] += v[i]
		}
		counts[p]++
	}
	for p, sum := range sums {
		if c := counts[p]; c > 1 {
			for i := range sum {
				sum[i] /= float32(c)
			}
		}
	}
	return sums
}

// Cosine returns the cosine similarity of two equal-length vectors; 0 when
// either is empty or their lengths differ.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Neighbors returns the top-k documents most cosine-similar to path, computed
// from the snapshot's vectors (no encoder). path itself is excluded; ties break
// by path ascending so the result is deterministic. It returns nil when the
// snapshot has no vector for path or is empty.
func (s *Snapshot) Neighbors(path string, k int) []Neighbor {
	docs := s.DocumentVectors()
	seed, ok := docs[path]
	if !ok || len(seed) == 0 {
		return nil
	}
	out := make([]Neighbor, 0, len(docs))
	for p, v := range docs {
		if p == path {
			continue
		}
		out = append(out, Neighbor{Path: p, Score: Cosine(seed, v)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	if k > 0 && len(out) > k {
		out = out[:k]
	}
	return out
}

// SimilarPair is one document pair whose cosine similarity crossed a threshold.
type SimilarPair struct {
	A     string
	B     string
	Score float64
}

// SimilarPairs returns the unordered pairs (in the given order) of the given
// paths whose cosine similarity is at least threshold, each reported once. Paths
// absent from the snapshot are skipped. It needs no encoder.
func (s *Snapshot) SimilarPairs(paths []string, threshold float64) []SimilarPair {
	docs := s.DocumentVectors()
	var out []SimilarPair
	for i := 0; i < len(paths); i++ {
		vi, ok := docs[paths[i]]
		if !ok {
			continue
		}
		for j := i + 1; j < len(paths); j++ {
			vj, ok := docs[paths[j]]
			if !ok {
				continue
			}
			if score := Cosine(vi, vj); score >= threshold {
				out = append(out, SimilarPair{A: paths[i], B: paths[j], Score: score})
			}
		}
	}
	return out
}
