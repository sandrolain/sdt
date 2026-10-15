package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── context knowledge index (reindex/lint) ─────────────────────────────────────

// ctxIndexTypeTier maps a context directory to its relevance tier. Essential
// tiers (architecture, decisions) sort first; history tiers last.
// sdtMarkdownExt avoids a repeated ".md" literal across context commands.

const sdtMarkdownExt = ".md"

// map keys reused by reindex/lint/template output.
const (
	ctxMapPath         = "path"
	ctxMapStatus       = "status"
	ctxTierEssential   = "essential"
	ctxTierImportant   = "important"
	ctxTierMedium      = "medium"
	ctxTierOperational = "operational"
	ctxTierHistory     = "history"
)

var ctxTierOrder = []string{ctxTierEssential, ctxTierImportant, ctxTierMedium, ctxTierOperational, ctxTierHistory}

func ctxTierForDir(dir string) string {
	if t, ok := ctxTypeForDir(dir); ok && t.tier != "" {
		return t.tier
	}
	return ctxTierHistory
}

const (
	ctxIndexRouteCorpus    = "corpus"
	ctxIndexRouteDedicated = "dedicated"
	ctxIndexRouteExcluded  = "excluded"
	ctxIndexRouteUnknown   = "unclassified"
)

// ctxIndexRouteForType classifies every registered kind. Wiki owns a dedicated
// schema/graph linter; tmp is scratch; other kinds enter the corpus walk when
// their registry entry promises an index tier or a status row. An unclassified
// result is a test failure, so a new registry kind cannot silently vanish.
func ctxIndexRouteForType(t ctxDocType) string {
	switch t.kind {
	case ctxTypeWiki:
		return ctxIndexRouteDedicated
	case ctxTypeTmp:
		return ctxIndexRouteExcluded
	default:
		if t.tier != "" || t.statusRow {
			return ctxIndexRouteCorpus
		}
		return ctxIndexRouteUnknown
	}
}

// ctxIndexDirsForTypes derives the corpus-wide index/lint walk from the type
// registry, in registry order.
func ctxIndexDirsForTypes(types []ctxDocType) []string {
	dirs := make([]string, 0, len(types))
	for _, t := range types {
		if ctxIndexRouteForType(t) != ctxIndexRouteCorpus {
			continue
		}
		dirs = append(dirs, t.dir)
	}
	return dirs
}

// ctxIndexDirs is the deterministic registry-order projection used by reindex,
// corpus-wide lint, store completeness and citation scans.
var ctxIndexDirs = ctxIndexDirsForTypes(contextTypeList)

// dirFiles returns the .md files under dir sorted by name, or nil when the
// directory does not exist.

func dirFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != sdtMarkdownExt {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// parseFrontmatterField returns the value of a top-level YAML frontmatter key,
// or "" when absent or malformed. Delegates to the shared contextwiki parser so
// the CLI and the viewer never drift.

func parseFrontmatterField(content, key string) string {
	return contextwiki.FrontmatterField(content, key)
}

// parseFrontmatterList parses a YAML block-list frontmatter field (a line
// `key:` followed by `  - item` lines), returning the list items. Falls back to
// a single inline value when present. Returns nil when the key is absent.
// Delegates to the shared contextwiki parser.

func parseFrontmatterList(content, key string) []string {
	return contextwiki.FrontmatterList(content, key)
}

// ctxResolvePath resolves a frontmatter reference ([[path]] or plain path)
// relative to the document's directory, returning the absolute path if it
// exists.

func ctxResolvePath(base, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, sdtMarkdownExt)
	if ref == "" {
		return "", false
	}
	abs := filepath.Join(base, ref)
	if !strings.HasSuffix(abs, sdtMarkdownExt) {
		abs += sdtMarkdownExt
	}
	if _, err := os.Stat(abs); err != nil { //#nosec G703 -- validated against context/ tree
		return "", false
	}
	return abs, true
}

// ctxResolveCorpusRef resolves a reference the way frontmatter stores one —
// corpus-relative, with or without the `context/` prefix the on-disk path
// carries — and returns the existing path. Both spellings are accepted because
// the stored form and the printed form differ: a caller copies
// `context/analysis/x.md` from a listing while the document holds
// `analysis/x.md`, and a verb that took only one of them would be wrong half the
// time.
func ctxResolveCorpusRef(ref string) (string, bool) {
	clean := filepath.ToSlash(strings.TrimSpace(ref))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, sdtWorkDir+"/")
	return ctxResolvePath(sdtWorkDir, clean)
}

func ctxIndexLine(dir, path string) string {
	rel, err := filepath.Rel(sdtWorkDir, path)
	if err != nil {
		rel = path
	}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return fmt.Sprintf("- [[%s]] — <unreadable>", rel)
	}
	summary := parseFrontmatterField(string(data), "summary")
	if summary == "" {
		summary = "<no summary>"
	}
	// Surface the controlled vocabulary in the index so topics/entities are
	// discoverable and feed prior-art review.
	if topics := parseFrontmatterList(string(data), "topics"); len(topics) > 0 {
		summary += " `topics: " + strings.Join(topics, ", ") + "`"
	}
	return fmt.Sprintf("- [[%s]] — %s", rel, summary)
}

// buildIndex renders the full context/index.md content grouped by tier.
