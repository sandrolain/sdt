package corpus

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// exclusionsTS is the client mirror of this package's exclusion set. The web
// bundle cannot import a Go package, so the two copies must be kept in sync by
// a gate. This test follows the delegate-skills relay-parity pattern: anchored
// extraction of the gated constants from the TS source, bidirectional equality
// with the Go sets, and an anti-smuggling pass that fails on any exclusion
// literal defined outside the gated constants.
var exclusionsTS = filepath.Join("..", "..", "web", "src", "lib", "exclusions.ts")

func TestExclusionParityWithWebMirror(t *testing.T) {
	src := readExclusionsTS(t)

	tsDirs := tsExcludedDirs(t, src)
	tsFiles := tsExcludedFiles(t, src)

	if !equalSets(excludedDirs, tsDirs) {
		t.Errorf("excluded dirs diverged:\n  go: %v\n  ts: %v\nexclusions.ts must mirror internal/corpus/corpus.go",
			sortedKeys(excludedDirs), sortedKeys(tsDirs))
	}
	if !equalSets(excludedFiles, tsFiles) {
		t.Errorf("excluded files diverged:\n  go: %v\n  ts: %v\nexclusions.ts must mirror internal/corpus/corpus.go",
			sortedKeys(excludedFiles), sortedKeys(tsFiles))
	}

	// commands/ is corpus content by design: it must never be excluded on either
	// side, otherwise the round-10 decision silently regresses.
	if _, ok := tsDirs["commands"]; ok {
		t.Error(`"commands" is excluded in exclusions.ts; commands/ is corpus content`)
	}
	if _, ok := excludedDirs["commands"]; ok {
		t.Error(`"commands" excluded in corpus.go; commands/ is corpus content`)
	}

	assertNoUngatedExclusions(t, src, tsDirs, tsFiles)
}

func readExclusionsTS(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(exclusionsTS)
	if err != nil {
		t.Fatalf("read %s: %v (the parity gate requires the web mirror)", exclusionsTS, err)
	}
	return string(src)
}

var tsSetLiteralRE = regexp.MustCompile(`(?s)EXCLUDED_DIRS\s*=\s*new Set\(\[([^\]]*)\]\)`)

func tsExcludedDirs(t *testing.T, src string) map[string]struct{} {
	t.Helper()
	m := tsSetLiteralRE.FindStringSubmatch(src)
	if m == nil {
		t.Fatal("EXCLUDED_DIRS set literal not found in exclusions.ts")
	}
	return parseQuotedStrings(t, m[1])
}

var tsPathVariableLiteralRE = regexp.MustCompile(`(?s)isExcludedPath\([^{]*\)\s*:[^{]*\{([^}]*)\}`)

func tsExcludedFiles(t *testing.T, src string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	for _, fn := range tsPathVariableLiteralRE.FindAllStringSubmatch(src, -1) {
		for _, lit := range quotedStrings(fn[1]) {
			if strings.HasPrefix(lit, ContextDir+"/") {
				out[lit] = struct{}{}
			}
		}
	}
	return out
}

var quotedStringRE = regexp.MustCompile(`"([^"]*)"`)

func parseQuotedStrings(t *testing.T, body string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	for _, p := range quotedStrings(body) {
		out[p] = struct{}{}
	}
	return out
}

func quotedStrings(body string) []string {
	raw := quotedStringRE.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(raw))
	for _, m := range raw {
		out = append(out, m[1])
	}
	return out
}

// assertNoUngatedExclusions fails when exclusions.ts references an exclusion
// path outside the two gated constants: a hardcoded "refs/" predicate or a
// second Set literal would silently bypass this gate.
func assertNoUngatedExclusions(t *testing.T, src string, dirs, files map[string]struct{}) {
	t.Helper()

	if got := strings.Count(src, "new Set("); got != 1 {
		t.Errorf("exclusions.ts contains %d new Set(...) literals, want exactly 1 (EXCLUDED_DIRS)", got)
	}

	stripped := tsSetLiteralRE.ReplaceAllString(src, "")
	stripped = tsPathVariableLiteralRE.ReplaceAllString(stripped, "")
	for name := range dirs {
		if strings.Contains(stripped, `"`+name+`"`) {
			t.Errorf("exclusion %q referenced outside EXCLUDED_DIRS in exclusions.ts (a hardcoded predicate bypasses the parity gate)", name)
		}
	}
	for p := range files {
		if strings.Contains(stripped, `"`+p+`"`) {
			t.Errorf("excluded file %q referenced outside the gated literals in exclusions.ts", p)
		}
	}
}

func equalSets(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
