// Package corpus centralizes which project-root-relative paths are excluded
// from the served knowledge base: the viewer tree and doc routes, the bleve
// search index and the wiki loader all consult it, so the exclusion set cannot
// drift between the server and the indexer.
package corpus

import (
	"path"
	"strings"
)

// ContextDir is the served knowledge base subdirectory under the project root.
const ContextDir = "context"

// excludedDirs are corpus subdirectory names never served, indexed or watched.
// tmp/ and scripts/ are working noise; refs/ is the large external clone;
// instructions/ is agent plumbing; sdtdocs/ is the generated per-command
// reference. commands/ is corpus content (command-trigger documents) and is
// intentionally NOT excluded.
//
// This set (and excludedFiles below) is mirrored in
// web/src/lib/exclusions.ts; parity_test.go enforces that the two never
// diverge.
var excludedDirs = map[string]struct{}{
	"tmp":          {},
	"scripts":      {},
	"refs":         {},
	"instructions": {},
	"sdtdocs":      {},
}

// excludedFiles are project-root-relative files never served or indexed.
var excludedFiles = map[string]struct{}{
	ContextDir + "/README.md": {},
}

// ExcludedDirName reports whether a directory name is excluded from the corpus.
func ExcludedDirName(name string) bool {
	_, ok := excludedDirs[name]
	return ok
}

// ExcludedPath reports whether a project-root-relative corpus path is excluded.
// Any excluded directory segment or an excluded index file matches; separators
// may be slash or OS-native.
func ExcludedPath(rel string) bool {
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(path.Clean(rel), "./")
	rel = strings.TrimPrefix(rel, "/")
	if _, ok := excludedFiles[rel]; ok {
		return true
	}
	for _, seg := range strings.Split(rel, "/") {
		if ExcludedDirName(seg) {
			return true
		}
	}
	return false
}
