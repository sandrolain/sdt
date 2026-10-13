package semantic

import (
	"reflect"
	"testing"
)

// TestClustersDeterministic guards the partition contract: the same document set
// always yields the same assignment (farthest-first seeding from the sorted-first
// document removes the run-to-run degree of freedom).
func TestClustersDeterministic(t *testing.T) {
	docs := twoGroups()
	first := Clusters(docs)
	for i := 0; i < 20; i++ {
		if got := Clusters(docs); !reflect.DeepEqual(got, first) {
			t.Fatalf("Clusters unstable at iteration %d: %v != %v", i, got, first)
		}
	}
}

// TestClustersSeparatesGroups checks the partition is meaningful: two
// well-separated families each stay together and land in different clusters.
func TestClustersSeparatesGroups(t *testing.T) {
	c := Clusters(map[string][]float32{
		"a1": {1, 0}, "a2": {0.99, 0.01}, "a3": {0.98, 0.02},
		"b1": {0, 1}, "b2": {0.01, 0.99}, "b3": {0.02, 0.98},
	})
	if c["a1"] != c["a2"] || c["a2"] != c["a3"] {
		t.Errorf("group A must stay together: %v", c)
	}
	if c["b1"] != c["b2"] || c["b2"] != c["b3"] {
		t.Errorf("group B must stay together: %v", c)
	}
	if c["a1"] == c["b1"] {
		t.Errorf("the two groups must land in different clusters: %v", c)
	}
}

// TestClustersDegenerate covers the boundary inputs: empty yields nil, a single
// document is cluster 0.
func TestClustersDegenerate(t *testing.T) {
	if Clusters(nil) != nil {
		t.Error("nil input must return nil")
	}
	if Clusters(map[string][]float32{}) != nil {
		t.Error("empty map must return nil")
	}
	one := Clusters(map[string][]float32{"only": {1, 0}})
	if len(one) != 1 || one["only"] != 0 {
		t.Errorf("single document must be cluster 0, got %v", one)
	}
}
