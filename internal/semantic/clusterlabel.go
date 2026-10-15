package semantic

import (
	"sort"
	"strconv"
)

// Cluster labelling. A cluster index means nothing to a reader ("cluster 3"), so
// the index is named from what it holds: the dominant document kind, plus the
// shortest member title when one kind clearly dominates. The rule is deterministic
// and needs no model — the members are ordered by path before anything is chosen,
// so the same input always yields the same labels.

// dominantShare is the fraction of a cluster's members one kind must hold for the
// cluster to be named after it with a representative title. Below it the cluster is
// mixed, and a title would claim a coherence the grouping does not have.
const dominantShare = 0.6

// labelTitleRunes bounds the representative title appended to a kind, so a long
// document title cannot dominate the legend it labels.
const labelTitleRunes = 32

// ClusterMember is one document inside a cluster: enough to name the cluster
// without holding the vectors.
type ClusterMember struct {
	Path  string
	Kind  string
	Title string
}

// ClusterLabels derives one label per cluster index. A cluster whose dominant kind
// holds at least `dominantShare` of its members is named `<Kind> · <shortest
// title>`; a mixed cluster is named `<Kind> <index>`, which names the plurality
// without pretending the group is one thing. An empty group is omitted, and a
// member with no kind counts as `unknown`.
func ClusterLabels(groups map[int][]ClusterMember) map[int]string {
	out := make(map[int]string, len(groups))
	for index, members := range groups {
		if len(members) == 0 {
			continue
		}
		sorted := append([]ClusterMember(nil), members...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

		counts := make(map[string]int, 4)
		for _, m := range sorted {
			counts[memberKind(m)]++
		}
		kind, count := dominantKind(counts)
		if float64(count)/float64(len(sorted)) < dominantShare {
			out[index] = kind + " " + strconv.Itoa(index)
			continue
		}
		if title := representativeTitle(sorted, kind); title != "" {
			out[index] = kind + " · " + title
			continue
		}
		out[index] = kind
	}
	return out
}

// memberKind is the member's kind, or `unknown` when the document declares none.
func memberKind(m ClusterMember) string {
	if m.Kind == "" {
		return "unknown"
	}
	return m.Kind
}

// dominantKind returns the most frequent kind and its count; ties break toward the
// lexicographically smallest kind, and no kind at all yields `unknown`.
func dominantKind(counts map[string]int) (string, int) {
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	best, bestCount := "unknown", 0
	for _, k := range kinds {
		if counts[k] > bestCount {
			best, bestCount = k, counts[k]
		}
	}
	return best, bestCount
}

// representativeTitle is the shortest title among the dominant kind's members,
// ties broken by path (already sorted), truncated to a bounded length. Empty when
// no member of that kind carries a title.
func representativeTitle(sorted []ClusterMember, kind string) string {
	best, bestLen := "", 0
	for _, m := range sorted {
		if memberKind(m) != kind {
			continue
		}
		title := m.Title
		if title == "" {
			continue
		}
		if best == "" || len([]rune(title)) < bestLen {
			best, bestLen = title, len([]rune(title))
		}
	}
	return truncateRunes(best, labelTitleRunes)
}

// truncateRunes bounds s to n runes, appending an ellipsis when it had to cut.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
