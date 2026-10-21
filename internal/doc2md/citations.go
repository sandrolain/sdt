package doc2md

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// citationLimit bounds how many citing paths one refusal names; the total is
// printed beside the list, so a heavily cited artifact is not truncated silently.
const citationLimit = 8

// readCorpusFile reads one candidate without failing the walk: an unreadable
// file simply cannot carry a citation, and one bad file must not hide the rest.
func readCorpusFile(path string) []byte {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus walk, never user input
	if err != nil {
		return nil
	}
	return data
}

// citationPattern builds the scanner for one kept artifact: the corpus cites an
// artifact by path and digest (`refs/converted/<file>@<sha8>`), and only the
// digest anchors a claim to the bytes, so the scan matches the path and the @.
func citationPattern(keptFile string) *regexp.Regexp {
	return regexp.MustCompile(`refs/converted/` + regexp.QuoteMeta(filepath.Base(keptFile)) + `@`)
}

// CitationsFor returns the sorted corpus paths that cite a kept artifact with
// the digest-anchored form. The scan is the rewrite guard's evidence: a rewrite
// that changes the artifact's bytes invalidates every citation it returns.
// The corpus root is the project root; the walk descends into `context/` only
// and skips the scratch and capture trees, where a citation target never lives.
func CitationsFor(root, keptFile string) ([]string, error) {
	pattern := citationPattern(keptFile)
	var findings []string
	err := filepath.WalkDir(filepath.Join(root, "context"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			switch filepath.Base(path) {
			case "refs", "tmp", "trash":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".markdown") {
			return nil
		}
		if !pattern.Match(readCorpusFile(path)) {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		findings = append(findings, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(findings)
	return findings, nil
}

// relToRoot prints a corpus path as the caller's messages do: relative to the
// project root, when the path is under it.
func relToRoot(root, path string) string {
	rel, err := filepath.Rel(filepath.Join(root, "context"), path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(filepath.Join("context", rel))
}

// corpusRoot walks up from the corpus directory (context/refs/converted) to the
// project root — the three directories above it — so the guard and the scan can
// see the documents that cite the artifacts it guards.
func corpusRoot(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(abs)))
}

// rewriteRefusal is the guard's one voice: why the write stops, and what a
// caller must name to force it.
func rewriteRefusal(path string, citations []string) error {
	switch {
	case len(citations) == 0:
		return fmt.Errorf(
			"refusing to rewrite %s with different bytes (no citation found in the corpus); pass --force to overwrite it", path)
	default:
		var b strings.Builder
		fmt.Fprintf(&b,
			"refusing to rewrite %s with different bytes: %d document citation(s) would go stale — pass --force to overwrite it:",
			path, len(citations))
		for i, c := range citations {
			if i == citationLimit {
				fmt.Fprintf(&b, "\n  … %d more", len(citations)-i)
				break
			}
			fmt.Fprintf(&b, "\n  %s", c)
		}
		return fmt.Errorf("%s", b.String())
	}
}
