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

// vcsDirs are pruned on every walk, outside the project's declaration. A VCS
// residue never holds content an agent is looking for and it is large, so a
// project that declares its scope must not be able to switch this off by
// omission: measured, declaring `skipDirs: ["dist"]` without the guard took the
// walked files from 2 to 6, every one of them inside `.git`.
var vcsDirs = []string{".git", ".hg", ".svn"}

// DefaultSkipDirs is the shipped scope: the VCS residues above plus the tool
// caches that carry no content an agent is looking for. A project replaces the
// **tool** part by declaring its own scope in its config; the VCS part is not
// replaceable.
var DefaultSkipDirs = append(append([]string{}, vcsDirs...), "node_modules", ".venv")

// nameSet turns a list of directory names into the lookup the descent uses.
func nameSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n != "" {
			set[n] = struct{}{}
		}
	}
	return set
}

// pruneMatcher decides whether a directory is inside the declared scope. It is
// path-based, not name-based, because a declared pattern is relative to the
// project root and keeps one meaning under `--dir`: the same file is
// `web/dist/big.js` from the project root and `dist/big.js` from `--dir web`.
//
// A pattern that names a path also covers everything beneath it, so `dist`
// prunes `dist/a/b` without having to be written `dist/**`.
type pruneMatcher struct {
	root     string
	patterns []*regexp.Regexp
}

// newPruneMatcher compiles the declared patterns. A pattern that is empty or does
// not compile is skipped rather than failing the search: a malformed declaration
// degrades to the rest of the scope, as every other config read here does.
func newPruneMatcher(root string, patterns []string) *pruneMatcher {
	m := &pruneMatcher{root: root}
	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		re, err := pruneRegexp(p)
		if err != nil || re == nil {
			continue
		}
		m.patterns = append(m.patterns, re)
	}
	return m
}

// pruneRegexp compiles one declared pattern with the subtree rule: it matches the
// path itself and everything beneath it. A pattern with no slash is a basename
// match at any depth, which is what a bare `node_modules` has to mean.
func pruneRegexp(pattern string) (*regexp.Regexp, error) {
	base, err := globRegexp(pattern)
	if err != nil || base == nil {
		return base, err
	}
	// Trim the `\z` globRegexp appends and close the subtree instead, so the
	// matcher has one contract whatever the pattern looks like: **this path, or
	// anything beneath it**. `dist` therefore covers `dist` and `dist/a/b`, while
	// neither `dist` nor `dist/**` ever reaches `distribution`.
	//
	// In a walk the `dist` hit prunes the subtree before the nested paths are
	// ever offered, so the broader form is belt and braces — but it is what
	// makes `matches` mean the same thing for a bare name as for a path.
	body := strings.TrimSuffix(base.String(), `\z`)
	return regexp.Compile(body + `(/.*)?\z`)
}

// searchScope resolves the name prune and the path prune for one walk. A
// declaration **replaces** the shipped names rather than extending them: a
// project that says `skipDirs: ["dist"]` is saying what its scope is, and
// silently keeping five built-ins it never asked about would make the
// declaration lie. With nothing declared, the shipped defaults apply and there is
// no project root to resolve patterns against.
func searchScope(projectRoot string, declared []string) (map[string]struct{}, *pruneMatcher) {
	// The VCS residues are pruned whatever the project said: a declaration
	// replaces the tool residues, never the control that keeps the walk out of
	// `.git`. Everything else is the project's to declare.
	names := nameSet(vcsDirs)
	if len(declared) == 0 {
		for _, n := range DefaultSkipDirs {
			names[n] = struct{}{}
		}
		return names, nil
	}
	for _, n := range declared {
		if n != "" {
			names[n] = struct{}{}
		}
	}
	return names, newPruneMatcher(projectRoot, declared)
}

// matches reports whether a walk-supplied directory is in the declared scope. It
// is false when the walk sits outside the project, because a project-root-relative
// pattern cannot be resolved from there.
func (m *pruneMatcher) matches(absDir string) bool {
	if m == nil || len(m.patterns) == 0 || m.root == "" {
		return false
	}
	rel, err := filepath.Rel(m.root, absDir)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	for _, re := range m.patterns {
		if re.MatchString(rel) {
			return true
		}
	}
	return false
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
var DefaultSkipPaths = []string{"context/refs", "context/tmp", "context/trash"}

// DefaultSkipPathsNoRefs is DefaultSkipPaths with the captured clones reachable.
// The scratch and trash directories stay pruned either way: one holds scratch by
// definition and the other holds what a caller discarded, and opting into a
// third-party clone is an opt-in to neither.
var DefaultSkipPathsNoRefs = []string{"context/tmp", "context/trash"}

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
	// ProjectRoot is the directory SkipDirs patterns are relative to. Empty means
	// the shipped defaults, which are directory names and need no root.
	ProjectRoot string
	// SkipDirs replaces the shipped name prune when non-empty: globs, relative to
	// ProjectRoot, pruning whole subtrees at the descent.
	SkipDirs []string
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
	names, declared := searchScope(opts.ProjectRoot, opts.SkipDirs)
	if err = walkAll(absRoot, names, pathSet(skip), declared, visit); err != nil {
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
	// ProjectRoot is the directory SkipDirs patterns are relative to. Empty means
	// the shipped defaults, which are directory names and need no root.
	ProjectRoot string
	// SkipDirs replaces the shipped name prune when non-empty: globs, relative to
	// ProjectRoot, pruning whole subtrees at the descent.
	SkipDirs []string
	// Pattern is a Go regular expression matched line by line.
	Pattern string
	// Path, when set, searches exactly that one file instead of the tree under
	// Root. It is the answer to "grep this file": the tree walk would need a
	// directory and a basename glob, which also matches the same name at any
	// depth. Root, Include and Exclude are refused by the caller rather than
	// silently ignored, and the reported path is the one the caller typed so the
	// answer round-trips into `sdt read -p`.
	Path string
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
	// Before and After carry that many non-matching lines adjacent to each
	// reported match. They are context, not matches: they never satisfy the
	// pattern and never count toward either cap.
	Before int
	After  int
	// FilesWithMatches answers "which files carry a match" instead of "where".
	// One entry per file is written to GrepResults.Paths and the file is not
	// read past its first match; MaxCountPerFile is meaningless in this mode
	// and the caller refuses it rather than letting it be ignored.
	FilesWithMatches bool
	// CountOnly reports how many lines match instead of returning them. The
	// count is a fact rather than a rendering, so no total cap may bound it —
	// the caller refuses one — while MaxCountPerFile still applies, because it
	// stops the scan of a file and `grep -m N -c` counts the same way.
	CountOnly bool
}

// GrepMatch is one matching line, with the adjacent lines the caller asked to
// see it in. The context is a field of the match rather than a sibling row, so a
// caller parses one shape per match and cannot pair a block with the wrong one.
type GrepMatch struct {
	Path   string        `json:"path"`
	Line   int           `json:"line"`
	Text   string        `json:"text"`
	Before []GrepContext `json:"before,omitempty"`
	After  []GrepContext `json:"after,omitempty"`
}

// GrepContext is one non-matching line adjacent to a match. It carries its own
// line number so a reader can address it with `sdt read --lines`.
type GrepContext struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// GrepResults is the reported set with the counters a caller needs to know the
// answer was bounded rather than complete.
type GrepResults struct {
	Matches []GrepMatch `json:"matches"`
	// Paths is the answer of the FilesWithMatches mode: the distinct files
	// carrying at least one match, sorted, with Matches left empty. It is a
	// field of its own rather than rows with empty line and text, so a caller
	// never has to read around placeholders to find the answer.
	Paths []string `json:"paths,omitempty"`
	// Count is the number of matching lines when CountOnly asked for a count,
	// and absent otherwise. It is a pointer because a count of zero is a real
	// answer that must not be confusable with "no count was requested".
	Count *int `json:"count,omitempty"`
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
	// A root that is not a directory cannot be searched: the walk would hand
	// back the file itself, rootRel would drop it as outside the root, and the
	// verb would answer "no match" with exit 0 — indistinguishable from an
	// empty tree. Refuse it with the same shape Find uses, so the two verbs do
	// not disagree about --dir.
	if st, statErr := os.Stat(absRoot); statErr != nil {
		return GrepResults{}, statErr
	} else if !st.IsDir() {
		return GrepResults{}, fmt.Errorf("grep: %s is not a directory", opts.Root)
	}
	s, err := newGrepScanner(absRoot, opts)
	if err != nil {
		return GrepResults{}, err
	}
	skip := opts.SkipPaths
	if skip == nil {
		skip = defaultSkipPaths
	}
	names, declared := searchScope(opts.ProjectRoot, opts.SkipDirs)
	if opts.Path != "" {
		if err := s.searchPath(opts.Path); err != nil {
			return GrepResults{}, err
		}
	} else if walkErr := walkAll(absRoot, names, pathSet(skip), declared, s.visit); walkErr != nil && !errors.Is(walkErr, errStopScan) {
		return GrepResults{}, walkErr
	}
	sortMatches(s.res.Matches)
	// The walk is lexical, so Paths comes out ordered already; the sort is the
	// same guarantee sortMatches gives, and keeps the two modes' output
	// contracts independent of the walk's order.
	sort.Strings(s.res.Paths)
	if opts.CountOnly && s.res.Count == nil {
		// A count of zero is an answer, not an absent field.
		s.res.Count = new(int)
	}
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

// searchPath scans one file instead of the tree. The reported path is the one
// the caller typed, so a match can be handed straight back to `sdt read -p`.
// A directory, a missing path or a binary file is answered rather than skipped:
// a single-file search that silently returns nothing is indistinguishable from
// a file with no matches.
func (s *grepScanner) searchPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("grep: %s is a directory; --dir searches a tree", path)
	}
	lines, binary, err := readTextLines(abs)
	if err != nil {
		return err
	}
	if binary {
		return nil // same rule as the walk: binary files are not read
	}
	s.res.FilesSearched++
	rel := filepath.ToSlash(filepath.Clean(path))
	n, scanErr := s.scanLines(rel, lines)
	if scanErr != nil && !errors.Is(scanErr, errStopScan) {
		return scanErr
	}
	if n > 0 {
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
	if s.opts.FilesWithMatches {
		return s.scanFilePresence(rel, lines)
	}
	if s.opts.CountOnly {
		return s.countLines(lines)
	}
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
		s.res.Matches = append(s.res.Matches, GrepMatch{
			Path:   rel,
			Line:   i + 1,
			Text:   line,
			Before: contextBefore(lines, i, s.opts.Before),
			After:  contextAfter(lines, i, s.opts.After),
		})
		perFile++
	}
	return perFile, nil
}

// scanFilePresence records the file and stops at its first match: the question
// is which files carry one, not where or how many. It returns 1 when the file
// was recorded, so the caller's FilesMatched counter keeps its meaning.
func (s *grepScanner) scanFilePresence(rel string, lines []string) (int, error) {
	for _, line := range lines {
		if !s.re.MatchString(line) {
			continue
		}
		if len(s.res.Paths) >= s.total {
			s.res.Truncated = true
			return 0, errStopScan
		}
		s.res.Paths = append(s.res.Paths, rel)
		return 1, nil
	}
	return 0, nil
}

// countLines counts the matching lines of one file without keeping them. A
// per-file cap stops that file's count, which is what `grep -m N -c` reports,
// and marks the answer truncated so a clipped count is never read as a total.
// No total cap applies: a bounded count is a lie rather than a display choice.
func (s *grepScanner) countLines(lines []string) (int, error) {
	n := 0
	for _, line := range lines {
		if s.opts.MaxCountPerFile > 0 && n >= s.opts.MaxCountPerFile {
			s.res.Truncated = true
			break
		}
		if s.re.MatchString(line) {
			n++
		}
	}
	if s.res.Count == nil {
		s.res.Count = new(int)
	}
	*s.res.Count += n
	return n, nil
}

// contextBefore collects the up to n lines preceding index i, in file order, so
// the block reads downward like the file it came from. It returns nil when there
// is nothing to collect, which keeps the field out of the JSON entirely.
func contextBefore(lines []string, i, n int) []GrepContext {
	if n <= 0 {
		return nil
	}
	from := max(0, i-n)
	if from == i {
		return nil
	}
	out := make([]GrepContext, 0, i-from)
	for j := from; j < i; j++ {
		out = append(out, GrepContext{Line: j + 1, Text: lines[j]})
	}
	return out
}

// contextAfter collects the up to n lines following index i, in file order.
func contextAfter(lines []string, i, n int) []GrepContext {
	if n <= 0 {
		return nil
	}
	to := min(len(lines), i+1+n)
	if to <= i+1 {
		return nil
	}
	out := make([]GrepContext, 0, to-i-1)
	for j := i + 1; j < to; j++ {
		out = append(out, GrepContext{Line: j + 1, Text: lines[j]})
	}
	return out
}

// errStopScan ends a Grep walk once the total cap is reached; it never escapes
// Grep as a failure.
var errStopScan = errors.New("grep: match cap reached")

// walkAll walks the raw tree in lexical order, skipping the VCS and dependency
// directories plus the caller's corpus prune. It is the only walk: it consults
// no ignore file, so every path outside those two sets is visited, including the
// ones .gitignore excludes.
func walkAll(root string, names, prune map[string]struct{}, declared *pruneMatcher, fn func(path string, d fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // an unreadable entry is skipped, not fatal
			return nil
		}
		if d.IsDir() && path != root {
			if _, skip := names[d.Name()]; skip {
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
			// The declared scope is matched on the project-root-relative path, so
			// one pattern means the same thing under every walk root.
			if declared.matches(path) {
				return filepath.SkipDir
			}
			// A surviving directory reaches the visitor like any other entry:
			// `find --dirs` reports them, and the grep visitor rejects them
			// through scannable.
			return fn(path, d)
		}
		return fn(path, d)
	})
}

// SplitLines splits text into its lines. A trailing newline terminates the last
// line rather than beginning an empty one, so the count does not depend on
// whether the file ends with a newline and an empty file has no lines at all.
//
// This is the one owner of that rule: the search scanner, `sdt read --lines` and
// `sdt read --count` all reach it, so a slice and a count cannot disagree about
// what the range contains.
func SplitLines(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if last := len(lines) - 1; last >= 0 && lines[last] == "" {
		lines = lines[:last]
	}
	return lines
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
	return SplitLines(string(data)), false, nil
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
