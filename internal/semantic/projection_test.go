package semantic

import (
	"math"
	"reflect"
	"testing"
)

// twoGroups is a well-separated 2D fixture shared by the projection and cluster
// tests: two near-orthogonal groups.
func twoGroups() map[string][]float32 {
	return map[string][]float32{
		"a1": {1, 0}, "a2": {0.98, 0.02},
		"b1": {0, 1}, "b2": {0.02, 0.98},
	}
}

func maxOf(xs []float64) float64 {
	m := math.Inf(-1)
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

func minOf(xs []float64) float64 {
	m := math.Inf(1)
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}

// TestProject2DDeterministic guards the layout contract: the same document set
// always yields the same coordinates (the fixed start vector and sign
// convention, and the sorted-path processing order, remove every run-to-run
// degree of freedom).
func TestProject2DDeterministic(t *testing.T) {
	docs := twoGroups()
	first := Project2D(docs)
	for i := 0; i < 20; i++ {
		if got := Project2D(docs); !reflect.DeepEqual(got, first) {
			t.Fatalf("Project2D unstable at iteration %d: %v != %v", i, got, first)
		}
	}
}

// TestProject2DSeparatesGroups checks the projection is meaningful, not just
// stable: two well-separated groups project apart on at least one component.
func TestProject2DSeparatesGroups(t *testing.T) {
	p := Project2D(twoGroups())
	separated := func(comp int) bool {
		var a, b []float64
		for id, xy := range p {
			if id[0] == 'a' {
				a = append(a, xy[comp])
			} else {
				b = append(b, xy[comp])
			}
		}
		return maxOf(a) < minOf(b) || maxOf(b) < minOf(a)
	}
	if !separated(0) && !separated(1) {
		t.Errorf("group separation not visible in the projection: %v", p)
	}
}

// TestProject2DDegenerate covers the no-variance fallbacks: empty input yields
// nil; a single document and an all-equal set project to the origin.
func TestProject2DDegenerate(t *testing.T) {
	if Project2D(nil) != nil {
		t.Error("nil input must return nil")
	}
	if Project2D(map[string][]float32{}) != nil {
		t.Error("empty map must return nil")
	}
	if got := Project2D(map[string][]float32{"only": {1, 2, 3}})["only"]; got != [2]float64{0, 0} {
		t.Errorf("single document must project to the origin, got %v", got)
	}
	for id, xy := range Project2D(map[string][]float32{"x": {1, 1}, "y": {1, 1}, "z": {1, 1}}) {
		if xy != [2]float64{0, 0} {
			t.Errorf("all-equal vectors must project to the origin, %s = %v", id, xy)
		}
	}
}
