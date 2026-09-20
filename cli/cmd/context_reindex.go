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
	b.WriteString("# context — Knowledge Index\n\n")
	b.WriteString("_Managed by `sdt context reindex`. Each row lists the file and its frontmatter `summary`._\n\n")
	for _, tier := range ctxTierOrder {
		rows, buckets, bucketSlugs, err := collectTierRows(tier)
		if err != nil {
			return "", err
		}
		if len(rows) == 0 && len(bucketSlugs) == 0 {
			continue
		}
		b.WriteString("## " + cases.Title(language.English).String(tier) + "\n\n")
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
		if len(rows) > 0 && len(bucketSlugs) > 0 {
			b.WriteString("\n")
		}
		sort.Strings(bucketSlugs)
		for _, slug := range bucketSlugs {
			b.WriteString("#### " + slug + "\n\n")
			for _, r := range buckets[slug] {
				b.WriteString(r + "\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// collectTierRows splits a relevance tier into the general rows and the
// per-objective buckets: Important analyses carrying an `objective` group key
// are bucketed, and dead-end notes (`note_type: dead-end` + `objective`) are
// surfaced in the same objective bucket. Everything else stays in the general
// list, except dead-end notes, which are never duplicated in the notes list.
func collectTierRows(tier string) (rows []string, buckets map[string][]string, bucketSlugs []string, err error) {
	buckets = map[string][]string{}
	for _, dir := range ctxIndexDirs {
		if ctxTierForDir(dir) != tier {
			continue
		}
		files, ferr := dirFiles(dir)
		if ferr != nil {
			return nil, nil, nil, ferr
		}
		for _, f := range files {
			kind, objective, noteType := ctxDocMeta(f)
			if kind == ctxTypeNotes && objective != "" && noteType == ctxNoteTypeDeadEnd {
				// Surfaced under the objective bucket, not in the notes list.
				continue
			}
			line := ctxIndexLine(dir, f)
			if tier == ctxTierImportant && kind == ctxTypeAnalysis && objective != "" {
				bucketSlugs = addObjectiveBucket(buckets, bucketSlugs, objective, line)
				continue
			}
			rows = append(rows, line)
		}
	}
	if tier == ctxTierImportant {
		bucketSlugs, err = collectDeadEndRows(buckets, bucketSlugs)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return rows, buckets, bucketSlugs, nil
}

// collectDeadEndRows appends the dead-end notes to their objective buckets,
// registering the bucket when no analysis opened it yet.
func collectDeadEndRows(buckets map[string][]string, bucketSlugs []string) ([]string, error) {
	files, err := dirFiles(sdtNotesDir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		kind, objective, noteType := ctxDocMeta(f)
		if kind != ctxTypeNotes || objective == "" || noteType != ctxNoteTypeDeadEnd {
			continue
		}
		bucketSlugs = addObjectiveBucket(buckets, bucketSlugs, objective, ctxDeadEndLine(f))
	}
	return bucketSlugs, nil
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
