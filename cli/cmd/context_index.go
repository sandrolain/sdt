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

// ctxIndexDirs lists the knowledge directories scanned by reindex, in a stable
// order per tier.

var ctxIndexDirs = []string{
	sdtArchitectureDir,
	sdtDecisionsDir,
	sdtAnalysisDir,
	sdtProposalsDir,
	sdtResearchDir,
	sdtPlanDir,
	sdtNotesDir,
	sdtQuestionsDir,
	sdtPromptsDir,
	sdtTasksDir,
	sdtCommandsDir,
	sdtWorklogDir,
	sdtArchiveDir,
}

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
	return fmt.Sprintf("- [[%s]] — %s", rel, summary)
}

// buildIndex renders the full context/index.md content grouped by tier.
