package cmd

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func buildIndex() (string, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: index\n")
	b.WriteString("summary: Generated knowledge index of context documents grouped by relevance tier\n")
	b.WriteString("_generated: auto\n")
	b.WriteString("---\n\n")
	b.WriteString("## context — Knowledge Index\n\n")
	b.WriteString("_Managed by `sdt context reindex`. Each row lists the file and its frontmatter `summary`._\n\n")

	// Cross-tier objective section: every document whose effective objective
	// resolves (analysis/plan `objective`, or a task inheriting its plan's) is
	// grouped here instead of its tier list.
	objectives, slugs, err := collectObjectiveBuckets()
	if err != nil {
		return "", err
	}
	sort.Strings(slugs)
	if len(slugs) > 0 {
		b.WriteString("## Objectives\n\n")
		for _, slug := range slugs {
			b.WriteString("### " + slug + "\n\n")
			for _, r := range objectives[slug] {
				b.WriteString(r + "\n")
			}
			b.WriteString("\n")
		}
	}

	for _, tier := range ctxTierOrder {
		rows, err := collectTierRows(tier)
		if err != nil {
			return "", err
		}
		if len(rows) == 0 {
			continue
		}
		b.WriteString("## " + cases.Title(language.English).String(tier) + "\n\n")
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
	}
	return b.String(), nil
}

// collectObjectiveBuckets groups every objective-tagged document (analysis,
// plan, or a task inheriting its plan's objective, plus dead-end notes tied to
// an objective) into one bucket per slug, regardless of tier. Bucket rows are
// sorted so the section is deterministic.
func collectObjectiveBuckets() (map[string][]string, []string, error) {
	buckets := map[string][]string{}
	var slugs []string
	for _, dir := range ctxIndexDirs {
		files, err := dirFiles(dir)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range files {
			kind, objective, noteType := ctxDocMeta(f)
			eff := ctxEffectiveObjective(f, kind, objective, noteType)
			if eff == "" {
				continue
			}
			line := ctxIndexLine(dir, f)
			if kind == ctxTypeNotes && noteType == ctxNoteTypeDeadEnd {
				line = ctxDeadEndLine(f)
			}
			slugs = addObjectiveBucket(buckets, slugs, eff, line)
		}
	}
	for slug := range buckets {
		sort.Strings(buckets[slug])
	}
	return buckets, slugs, nil
}

// ctxEffectiveObjective resolves the objective a document groups under:
// analysis/plan read their own `objective`; a task inherits its plan's; a note
// only when it is a dead-end. Other kinds (and ungrouped docs) return "".
func ctxEffectiveObjective(path, kind, objective, noteType string) string {
	switch kind {
	case ctxTypeAnalysis, ctxTypePlan:
		return objective
	case ctxTypeTasks:
		return ctxTaskPlanObjective(path)
	case ctxTypeNotes:
		if noteType == ctxNoteTypeDeadEnd {
			return objective
		}
	}
	return ""
}

// ctxTaskPlanObjective reads the objective of the plan a task file sources
// (`sources`, falling back to `links`); "" when the plan is missing or carries
// none.
func ctxTaskPlanObjective(path string) string {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return ""
	}
	content := string(data)
	refs := parseFrontmatterList(content, ctxFrontmatterSources)
	if len(refs) == 0 {
		refs = parseFrontmatterList(content, ctxFrontmatterLinks)
	}
	for _, ref := range refs {
		abs, ok := ctxResolvePath(sdtWorkDir, ref)
		if !ok {
			continue
		}
		pdata, perr := os.ReadFile(abs) //#nosec G304 -- path resolved within context/
		if perr != nil {
			continue
		}
		pc := string(pdata)
		if parseFrontmatterField(pc, "kind") != ctxTypePlan {
			continue
		}
		if o := parseFrontmatterField(pc, "objective"); o != "" {
			return o
		}
	}
	return ""
}

// collectTierRows returns the general rows of a relevance tier: documents
// without an effective objective. Objective-tagged documents are rendered by
// collectObjectiveBuckets instead, so they never appear in a tier list.
func collectTierRows(tier string) ([]string, error) {
	var rows []string
	for _, dir := range ctxIndexDirs {
		if ctxTierForDir(dir) != tier {
			continue
		}
		files, err := dirFiles(dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			kind, objective, noteType := ctxDocMeta(f)
			if ctxEffectiveObjective(f, kind, objective, noteType) != "" {
				continue
			}
			rows = append(rows, ctxIndexLine(dir, f))
		}
	}
	return rows, nil
}

// addObjectiveBucket appends a line to an objective bucket, registering the
// slug once and returning the (possibly extended) ordered slug list.
func addObjectiveBucket(buckets map[string][]string, bucketSlugs []string, objective, line string) []string {
	if _, seen := buckets[objective]; !seen {
		bucketSlugs = append(bucketSlugs, objective)
	}
	buckets[objective] = append(buckets[objective], line)
	return bucketSlugs
}

// ctxDeadEndLine renders a dead-end note index row with an explicit marker.
func ctxDeadEndLine(path string) string {
	line := ctxIndexLine(sdtNotesDir, path)
	return strings.Replace(line, " — ", " — **dead-end** ", 1)
}

// ctxDocMeta returns the frontmatter kind, optional `objective` group key and
// optional `note_type` of a context document.
func ctxDocMeta(path string) (kind, objective, noteType string) {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return "", "", ""
	}
	content := string(data)
	return parseFrontmatterField(content, "kind"),
		parseFrontmatterField(content, "objective"),
		parseFrontmatterField(content, "note_type")
}

func writeIndex(content string) error {
	if err := os.MkdirAll(sdtWorkDir, 0o750); err != nil { //#nosec G301 -- user work dir
		return err
	}
	//#nosec G306 -- user work file
	return os.WriteFile(sdtContextIndex, []byte(content), 0o644)
}

var contextReindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Regenerate context/index.md from frontmatter summaries",
	Long: `Scan the context/ knowledge directories, read the mandatory frontmatter
summary of every document and regenerate context/index.md grouped by
relevance tier (essential, important, medium, operational, history).

Examples:
  sdt context reindex
  sdt context reindex --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := buildIndex()
		exitWithError(cmd, err)
		written, err := writeIndexIfChanged(content)
		exitWithError(cmd, err)
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(map[string]string{ctxMapPath: sdtContextIndex, ctxMapStatus: written}, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(map[string]string{ctxMapPath: sdtContextIndex, ctxMapStatus: written})
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, sdtContextIndex+" ("+written+")\n")
		}
	},
}

func writeIndexIfChanged(content string) (string, error) {
	if data, err := os.ReadFile(sdtContextIndex); err == nil && string(data) == content {
		return "skipped", nil
	}
	if err := writeIndex(content); err != nil {
		return "", err
	}
	return "written", nil
}

// ctxLintIssue is one validation finding from lint.
