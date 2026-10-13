package semantic

import (
	"math"
	"sort"
)

// Power-iteration controls for the 2D PCA. A fixed iteration bound keeps the
// result deterministic even when the two components are nearly tied; the early
// exit on convergence keeps well-separated data cheap. No dependency is used —
// the routines are hand-rolled so the determinism contract is explicit.
const (
	projectionIterations = 200
	projectionTolerance  = 1e-12
)

// Project2D computes a deterministic 2D PCA of docs (path -> document vector):
// the top two principal components via power iteration on the centred Gram
// matrix (kernel PCA), so cost is O(n^2 * dim) and independent of a covariance
// of the full dimension. It needs no encoder — the persisted vectors only.
//
// It returns nil for an empty input. A single document, or a degenerate
// (all-equal) set, has no variance and projects every point to the origin, which
// is the honest deterministic answer. The sign of each component is fixed (sum
// positive) and documents are processed in sorted-path order, so the same
// snapshot always yields the same layout.
func Project2D(docs map[string][]float32) map[string][2]float64 {
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

	centroid := make([]float64, dim)
	for _, id := range ids {
		v := docs[id]
		for j := 0; j < dim; j++ {
			centroid[j] += float64(v[j])
		}
	}
	for j := range centroid {
		centroid[j] /= float64(n)
	}

	centred := make([][]float64, n)
	for i, id := range ids {
		v := docs[id]
		row := make([]float64, dim)
		for j := 0; j < dim; j++ {
			row[j] = float64(v[j]) - centroid[j]
		}
		centred[i] = row
	}

	gram := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			var s float64
			for d := 0; d < dim; d++ {
				s += centred[i][d] * centred[j][d]
			}
			gram[i*n+j] = s
			gram[j*n+i] = s
		}
	}

	u1, l1 := topEigenvector(gram, n)
	u2, l2 := topEigenvector(deflate(gram, n, u1, l1), n)
	s1, s2 := math.Sqrt(math.Max(l1, 0)), math.Sqrt(math.Max(l2, 0))

	out := make(map[string][2]float64, n)
	for i, id := range ids {
		out[id] = [2]float64{u1[i] * s1, u2[i] * s2}
	}
	return out
}

// topEigenvector returns the dominant eigenvector of the symmetric matrix k
// (row-major n x n) and its eigenvalue, by power iteration from a fixed
// all-ones start. A zero operator (no variance) yields the zero vector with
// eigenvalue 0. The sign is fixed so the result is reproducible.
func topEigenvector(k []float64, n int) ([]float64, float64) {
	u := make([]float64, n)
	for i := range u {
		u[i] = 1
	}
	if !normalize(u) {
		return u, 0
	}
	for iter := 0; iter < projectionIterations; iter++ {
		next := make([]float64, n)
		for i := 0; i < n; i++ {
			row := k[i*n : (i+1)*n]
			var s float64
			for j, v := range row {
				s += v * u[j]
			}
			next[i] = s
		}
		if !normalize(next) {
			return make([]float64, n), 0 // zero operator: no variance to spread
		}
		var diff float64
		for i := range next {
			diff += math.Abs(next[i] - u[i])
		}
		u = next
		if diff < projectionTolerance {
			break
		}
	}
	var lambda float64 // Rayleigh quotient == eigenvalue for a unit u
	for i := 0; i < n; i++ {
		row := k[i*n : (i+1)*n]
		for j, v := range row {
			lambda += u[i] * v * u[j]
		}
	}
	fixEigenSign(u)
	return u, lambda
}

// deflate subtracts the rank-one component (u, lambda) from k, so the next power
// iteration yields the runner-up principal component.
func deflate(k []float64, n int, u []float64, lambda float64) []float64 {
	out := make([]float64, len(k))
	copy(out, k)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			out[i*n+j] -= lambda * u[i] * u[j]
		}
	}
	return out
}

// fixEigenSign flips u so its entries sum non-negative; an eigenvector's sign is
// arbitrary, so this makes it reproducible.
func fixEigenSign(u []float64) {
	var sum float64
	for _, v := range u {
		sum += v
	}
	if sum < 0 {
		for i := range u {
			u[i] = -u[i]
		}
	}
}

// normalize scales u to unit length in place, reporting false when u is zero.
func normalize(u []float64) bool {
	var norm float64
	for _, v := range u {
		norm += v * v
	}
	if norm == 0 {
		return false
	}
	norm = math.Sqrt(norm)
	for i := range u {
		u[i] /= norm
	}
	return true
}

// commonDim returns the smallest vector length across ids (so every row shares a
// dimension), or 0 when any vector is empty.
func commonDim(docs map[string][]float32, ids []string) int {
	dim := -1
	for _, id := range ids {
		l := len(docs[id])
		if l == 0 {
			return 0
		}
		if dim < 0 || l < dim {
			dim = l
		}
	}
	if dim < 0 {
		return 0
	}
	return dim
}
