package cmd

import (
	"encoding/json"
	"os"
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
		var rows []string
		for _, dir := range ctxIndexDirs {
			if ctxTierForDir(dir) != tier {
				continue
			}
			files, err := dirFiles(dir)
			if err != nil {
				return "", err
			}
			for _, f := range files {
				rows = append(rows, ctxIndexLine(dir, f))
			}
		}
		if len(rows) == 0 {
			continue
		}
		b.WriteString("## " + cases.Title(language.English).String(tier) + "\n\n")
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
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
