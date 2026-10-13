package semantic

import (
	"math"
	"sort"
)

// Cluster controls. k is derived from n and capped so a small corpus does not
// fragment into singletons; the iteration bound keeps the loop finite while the
// assignment stabilisation is the real stop.
const (
	clusterMaxK  = 8
	clusterIters = 50
)

// Clusters partitions docs (path -> document vector) into deterministic groups
// by cosine similarity, returning path -> cluster index. Initialisation is
// farthest-first from the lexicographically first document, so the partition
// depends only on the input; ties break toward the smallest index or path. It
// needs no encoder and returns nil for an empty input; a single document is
// cluster 0.
func Clusters(docs map[string][]float32) map[string]int {
	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	n := len(ids)
	if n == 0 {
		return nil
	}
	dim := commonDim(docs, ids)
	if dim == 0 {
		return nil
	}
	vecs := make([][]float32, n)
	for i, id := range ids {
		vecs[i] = docs[id][:dim]
	}

	k := clusterCount(n)

	// Farthest-first seeding: the first document, then repeatedly the point
	// farthest from the chosen set.
	centroids := make([][]float32, 0, k)
	chosen := make([]bool, n)
	centroids = append(centroids, vecs[0])
	chosen[0] = true
	for len(centroids) < k {
		best, bestDist := -1, -1.0
		for i := range vecs {
			if chosen[i] {
				continue
			}
			if d := nearestSimDistance(vecs[i], centroids); d > bestDist {
				bestDist, best = d, i
			}
		}
		if best < 0 {
			break
		}
		chosen[best] = true
		centroids = append(centroids, vecs[best])
	}
	k = len(centroids)

	assign := make([]int, n)
	for i := range assign {
		assign[i] = -1
	}
	for iter := 0; iter < clusterIters; iter++ {
		changed := false
		for i := range vecs {
			c := nearestCluster(vecs[i], centroids)
			if assign[i] != c {
				assign[i] = c
				changed = true
			}
		}
		if !changed {
			break
		}
		centroids = recentre(vecs, assign, centroids, k, dim)
	}

	out := make(map[string]int, n)
	for i, id := range ids {
		out[id] = assign[i]
	}
	return out
}

// clusterCount derives the group count from n (a square-root rule of thumb),
// clamped to [2, clusterMaxK] and never above n.
func clusterCount(n int) int {
	if n < 2 {
		return 1
	}
	k := int(math.Round(math.Sqrt(float64(n) / 2)))
	if k < 2 {
		k = 2
	}
	if k > clusterMaxK {
		k = clusterMaxK
	}
	if k > n {
		k = n
	}
	return k
}

// nearestCluster returns the index of the centroid most cosine-similar to v;
// ties break toward the smallest index.
func nearestCluster(v []float32, centroids [][]float32) int {
	best, bestSim := 0, -2.0
	for c, cent := range centroids {
		if s := Cosine(v, cent); s > bestSim {
			bestSim, best = s, c
		}
	}
	return best
}

// nearestSimDistance returns the smallest cosine distance (1 - similarity) from
// v to any centroid.
func nearestSimDistance(v []float32, centroids [][]float32) float64 {
	best := 2.0
	for _, cent := range centroids {
		if d := 1 - Cosine(v, cent); d < best {
			best = d
		}
	}
	return best
}

// recentre recomputes each centroid as the mean of its members. An empty cluster
// keeps its previous centroid, so the iteration stays deterministic.
func recentre(vecs [][]float32, assign []int, prev [][]float32, k, dim int) [][]float32 {
	sums := make([][]float64, k)
	counts := make([]int, k)
	for i, v := range vecs {
		c := assign[i]
		if sums[c] == nil {
			sums[c] = make([]float64, dim)
		}
		for j := range v {
			sums[c][j] += float64(v[j])
		}
		counts[c]++
	}
	out := make([][]float32, k)
	for c := 0; c < k; c++ {
		if counts[c] == 0 {
			out[c] = prev[c]
			continue
		}
		cent := make([]float32, dim)
		for j := range cent {
			cent[j] = float32(sums[c][j] / float64(counts[c]))
		}
		out[c] = cent
	}
	return out
}
