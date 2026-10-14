//go:build !wasm

package search

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// contentsearch answers the two file-locating questions an agent asks with a
// host tool today: which paths match a glob, and which files contain a pattern.
// The walk covers the raw working tree and consults no ignore file: `.gitignore`
// answers "what would git commit", which is not the question a reader is asking,
// and letting it decide visibility hid the whole context/ corpus behind a
// version-control choice. Scope is owned here instead, through skipDirs and the
// per-walk skip set.

// skipDirs are never walked: version-control and tool caches carry no content an
// agent is looking for, and their size makes an unbounded scan expensive.
var skipDirs = map[string]struct{}{
	".git":         {},
	".hg":          {},
	".svn":         {},
	"node_modules": {},
	".venv":        {},
}

// DefaultSkipPaths is the corpus scope a walk prunes unless the caller widens
// it, given as slash-separated paths relative to the walk root. The corpus of
// work is deliberately **not** pruned: it is the tree an agent reads, and it is
// small (59 MB on a mature corpus). `context/refs/` is pruned because it holds
// third-party clones — 9,2 GB and 131k files on the same corpus — and a
// directory that large cannot sit in the path of every search.
//
// Pruning by **path** rather than by directory name is deliberate: a source
// directory that happens to be called `refs` or `tmp` stays searchable, and only
// the corpus's own clones are skipped.
var DefaultSkipPaths = []string{"context/refs", "context/tmp"}

// DefaultSkipPathsNoRefs is DefaultSkipPaths with the captured clones reachable.
// The scratch directory stays pruned either way: it holds scratch by definition,
// and opting into a third-party clone is not an opt-in to scratch.
var DefaultSkipPathsNoRefs = []string{"context/tmp"}

// defaultSkipPaths is the internal alias for the nil-means-default contract.
var defaultSkipPaths = DefaultSkipPaths

// pathSet normalises a prune list into a lookup set, dropping empty entries and
// the trailing separator so a caller may write either form.
func pathSet(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		p = strings.TrimSuffix(filepath.ToSlash(p), "/")
		if p != "" && p != "." {
			set[p] = struct{}{}
		}
	}
	return set
}

// binarySniffBytes is how much of a file is inspected to classify it. A NUL byte
// in the first block is the conventional binary signal (git's too).
const binarySniffBytes = 8000

// FindOptions bounds a path-glob search.
type FindOptions struct {
	// Root is the directory the glob is evaluated against. Empty means the
	// working directory.
	Root string
	// SkipPaths prunes subtrees by their slash-separated path relative to Root,
	// before the glob is applied. Nil means DefaultSkipPaths; an empty non-nil
	// slice prunes nothing, which is how a caller widens the walk to everything.
	SkipPaths []string
	// Pattern is a glob over slash-separated paths relative to Root. An empty
	// pattern matches every path.
	Pattern string
	// IncludeDirs keeps directories in the result; the default is files only.
	IncludeDirs bool
	// Max caps the result set; zero means the package default.
	Max int
}

// FindResult is one matched path with the marker a caller needs to tell a file
// from a directory without a second stat.
type FindResult struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir,omitempty"`
}

// resolveRoot returns the absolute search root, defaulting to the working
// directory. Both content verbs resolve it the same way, so a path reported by
// `find` and a path reported by `grep` mean the same thing.
func resolveRoot(root string) (string, error) {
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	return filepath.Abs(root)
}

// Find returns every path under the root matching the glob, in sorted order. The
// order is deterministic by construction (the walk is sorted, or sorted
// afterwards) so two runs on an unchanged tree produce identical output.
func Find(opts FindOptions) ([]FindResult, error) {
	absRoot, err := resolveRoot(opts.Root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("find: %s is not a directory", opts.Root)
	}
	var match *regexp.Regexp
	if opts.Pattern != "" {
		match, err = globRegexp(opts.Pattern)
		if err != nil {
			return nil, err
		}
	}
	seen := map[string]struct{}{}
	visit := func(path string, d fs.DirEntry) error {
		rel, rerr := rootRel(absRoot, path)
		if rerr != nil {
			return nil // outside the root: not a result, not a failure
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, dup := seen[rel]; dup {
			return nil
		}
		if d.IsDir() && !opts.IncludeDirs {
			return nil
		}
		if match != nil && !match.MatchString(rel) {
			return nil
		}
		seen[rel] = struct{}{}
		return nil
	}
	skip := opts.SkipPaths
	if skip == nil {
		skip = defaultSkipPaths
	}
	if err = walkAll(absRoot, pathSet(skip), visit); err != nil {
		return nil, err
	}
	out := make([]FindResult, 0, len(seen))
	for rel := range seen {
		out = append(out, FindResult{Path: rel, IsDir: isDir(filepath.Join(absRoot, filepath.FromSlash(rel)))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return !out[i].IsDir && out[j].IsDir
	})
	if opts.Max > 0 && len(out) > opts.Max {
		out = out[:opts.Max]
	}
	return out, nil
}

// GrepOptions bounds a content search.
type GrepOptions struct {
	// Root is the directory to search. Empty means the working directory.
	Root string
	// Pattern is a Go regular expression matched line by line.
	Pattern string
	// Include and Exclude are globs over slash-separated paths relative to Root;
	// either may be empty. Exclude wins over Include.
	Include string
	Exclude string
	// SkipPaths prunes subtrees by their slash-separated path relative to Root,
	// before the include and exclude filters are applied. Nil means
	// DefaultSkipPaths; an empty non-nil slice prunes nothing, which is how a
	// caller widens the walk to everything.
	SkipPaths []string
	// MaxCountPerFile caps the reported matches per file; zero means unlimited.
	MaxCountPerFile int
	// Max caps the total reported matches; zero means the package default.
	Max int
	// CaseInsensitive lowercases the pattern before compiling it.
	CaseInsensitive bool
}

// GrepMatch is one matching line.
type GrepMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// GrepResults is the reported set with the counters a caller needs to know the
// answer was bounded rather than complete.
type GrepResults struct {
	Matches []GrepMatch `json:"matches"`
	// FilesMatched counts the files carrying at least one match.
	FilesMatched int `json:"filesMatched"`
	// FilesSearched counts the text files actually read.
	FilesSearched int `json:"filesSearched"`
	// Truncated is true when a cap stopped the scan before the tree was read.
	Truncated bool `json:"truncated,omitempty"`
}

// grepDefaultMax is the total-match cap applied when a caller sets none. The
// output of a verb an agent reads into a context window must be bounded by
// default; --max 0 asks for the whole tree explicitly.
const grepDefaultMax = 200

// Grep returns the lines matching the pattern across the tracked tree, in
// stable path-then-line order. Binary files, VCS directories and dependency
// trees are skipped without being counted, and both a per-file and a total cap
// are applied so a one-token pattern cannot return the repository.
func Grep(opts GrepOptions) (GrepResults, error) {
	absRoot, err := resolveRoot(opts.Root)
	if err != nil {
		return GrepResults{}, err
	}
	s, err := newGrepScanner(absRoot, opts)
	if err != nil {
		return GrepResults{}, err
	}
	skip := opts.SkipPaths
	if skip == nil {
		skip = defaultSkipPaths
	}
	if walkErr := walkAll(absRoot, pathSet(skip), s.visit); walkErr != nil && !errors.Is(walkErr, errStopScan) {
		return GrepResults{}, walkErr
	}
	sortMatches(s.res.Matches)
	return s.res, nil
}

// sortMatches orders the reported lines by path then line number, so the
// reported order does not depend on the walk order.
func sortMatches(matches []GrepMatch) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Line < matches[j].Line
	})
}

// grepScanner carries the compiled filters, the resolved caps and the running
// counters of one Grep run. Keeping them together means the per-file visit is a
// few lines and each decision (is this a file to scan? does it count? is the
// cap reached?) is readable on its own.
type grepScanner struct {
	absRoot string
	opts    GrepOptions
	re      *regexp.Regexp
	include *regexp.Regexp
	exclude *regexp.Regexp
	total   int
	res     GrepResults
}

// newGrepScanner compiles the pattern and path filters and resolves the caps. A
// zero total means the package default; a negative total means unbounded.
func newGrepScanner(absRoot string, opts GrepOptions) (*grepScanner, error) {
	expr := opts.Pattern
	if opts.CaseInsensitive {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	include, err := globRegexp(opts.Include)
	if err != nil {
		return nil, err
	}
	exclude, err := globRegexp(opts.Exclude)
	if err != nil {
		return nil, err
	}
	total := opts.Max
	if total == 0 {
		total = grepDefaultMax
	}
	if total < 0 {
		total = math.MaxInt
	}
	return &grepScanner{
		absRoot: absRoot,
		opts:    opts,
		re:      re,
		include: include,
		exclude: exclude,
		total:   total,
		res:     GrepResults{Matches: []GrepMatch{}},
	}, nil
}

// visit scans one walked entry, keeping errStopScan as the signal that the total
// cap was reached.
func (s *grepScanner) visit(path string, d fs.DirEntry) error {
	rel, ok := s.scannable(path, d)
	if !ok {
		return nil
	}
	lines, binary, err := readTextLines(resolvePath(s.absRoot, path))
	if err != nil || binary {
		// An unreadable or binary file is skipped, never fatal: the rest of the
		// tree still answers the query.
		return nil
	}
	s.res.FilesSearched++
	perFile, err := s.scanLines(rel, lines)
	if err != nil {
		return err // the cap ended the scan: this file is not counted as matched
	}
	if perFile > 0 {
		s.res.FilesMatched++
	}
	return nil
}

// scannable reports the root-relative path of an entry that is a file passing
// the include/exclude filters; every other entry is not scanned.
func (s *grepScanner) scannable(path string, d fs.DirEntry) (string, bool) {
	rel, err := rootRel(s.absRoot, path)
	if err != nil || (d != nil && d.IsDir()) {
		return "", false
	}
	if s.include != nil && !s.include.MatchString(rel) {
		return "", false
	}
	if s.exclude != nil && s.exclude.MatchString(rel) {
		return "", false
	}
	return rel, true
}

// scanLines appends every matching line of one file, honouring the per-file cap
// and the total cap. It returns the number of matches appended, and errStopScan
// once the total cap is reached.
func (s *grepScanner) scanLines(rel string, lines []string) (int, error) {
	perFile := 0
	for i, line := range lines {
		if s.opts.MaxCountPerFile > 0 && perFile >= s.opts.MaxCountPerFile {
			s.res.Truncated = true
			return perFile, nil
		}
		if !s.re.MatchString(line) {
			continue
		}
		if len(s.res.Matches) >= s.total {
			s.res.Truncated = true
			return perFile, errStopScan
		}
		s.res.Matches = append(s.res.Matches, GrepMatch{Path: rel, Line: i + 1, Text: line})
		perFile++
	}
	return perFile, nil
}

// errStopScan ends a Grep walk once the total cap is reached; it never escapes
// Grep as a failure.
var errStopScan = errors.New("grep: match cap reached")

// walkAll walks the raw tree in lexical order, skipping the VCS and dependency
// directories. It is the `--no-ignore` path: every other file is visited,
// including the ones .gitignore excludes.
func walkAll(root string, prune map[string]struct{}, fn func(path string, d fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // an unreadable entry is skipped, not fatal
			return nil
		}
		if d.IsDir() && path != root {
			if _, skip := skipDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			// A corpus-scope prune is matched on the path relative to the walk
			// root, so it removes the subtree instead of filtering its results:
			// `--exclude` cannot do this, it leaves the descent untouched.
			if len(prune) > 0 {
				if rel, rerr := rootRel(root, path); rerr == nil {
					if _, skip := prune[rel]; skip {
						return filepath.SkipDir
					}
				}
			}
			// A surviving directory reaches the visitor like any other entry:
			// `find --dirs` reports them, and the grep visitor rejects them
			// through scannable.
			return fn(path, d)
		}
		return fn(path, d)
	})
}

// readTextLines reads a file's lines and reports whether it looks binary.
func readTextLines(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- the path comes from a directory walk of the requested root
	if err != nil {
		return nil, false, err
	}
	if bytes.IndexByte(data[:min(len(data), binarySniffBytes)], 0) >= 0 {
		return nil, true, nil
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), false, nil
}

// isDir reports whether path is a directory, false on any error.
func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// rootRel normalizes a walk path to a slash-separated root-relative path. A
// walker may hand back either an absolute path or one relative to the root, and
// the reported paths must be identical in both cases or the same file is
// reported under two names.
func rootRel(absRoot, path string) (string, error) {
	rel, err := filepath.Rel(absRoot, resolvePath(absRoot, path))
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "", errOutsideRoot
	}
	return filepath.ToSlash(rel), nil
}

// resolvePath turns a walker-supplied path into an absolute one, so a file can
// be read regardless of the working directory the walk ran from.
func resolvePath(absRoot, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(absRoot, filepath.FromSlash(path))
}

// errOutsideRoot marks a walk entry that is the root itself or lives outside it.
var errOutsideRoot = errors.New("path is outside the root")

// globRegexp compiles a shell-style glob into an anchored whole-path regexp.
// `**` spans separators and `*` does not, which is the distinction a glob caller
// expects and which filepath.Match alone cannot express.
//
// A pattern without a separator matches a **basename at any depth** — `*.md`
// finds every markdown file, the way `find -name` behaves — while a pattern
// containing `/` is matched against the whole root-relative path. The rule is
// stated rather than implied so a caller can predict it without a test.
func globRegexp(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	basename := !strings.ContainsRune(pattern, '/')
	var b strings.Builder
	// Every compiled glob is anchored at the start. A pattern carrying a slash is
	// documented as matching the whole path, so `context/analysis/*.md` must not
	// match `cli/cmd/testdata/…/context/analysis/normal.md` — an unanchored
	// regexp matches any suffix, which let a test fixture answer a corpus glob
	// and fill the --limit before a single real document was reached.
	if basename {
		b.WriteString(`\A(?:.*/)?`)
	} else {
		b.WriteString(`\A`)
	}
	for i := 0; i < len(pattern); {
		switch {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString(`(?:[^/]+/)*`)
			i += 3
		case pattern[i] == '*':
			b.WriteString(`[^/]*`)
			i++
		case pattern[i] == '?':
			b.WriteString(`[^/]`)
			i++
		case strings.ContainsRune(`\.+()|[]{}^$`, rune(pattern[i])):
			b.WriteString(`\`)
			b.WriteByte(pattern[i])
			i++
		default:
			b.WriteByte(pattern[i])
			i++
		}
	}
	b.WriteString(`\z`)
	return regexp.Compile(b.String())
}
