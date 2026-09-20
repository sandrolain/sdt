package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func outputStringList(cmd *cobra.Command, items []string) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(items, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(items)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, it := range items {
			outputString(cmd, it+"\n")
		}
	}
}

func listContextFiles(dir string) ([]string, error) {
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

var contextListCmd = &cobra.Command{
	Use:   useList,
	Short: "List context/ work files",
	Long: `List existing work files under context/ for a type, sorted by name
(chronological for timestamped files).

Types: ` + ctxListHelpText() + `.

Examples:
  sdt context list --type worklog
  sdt context list --type decision --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		if typ == ctxDocAliasDecision {
			typ = ctxTypeDecision
		}
		t, ok := ctxTypeLookup(typ)
		if !ok || !t.listSupported {
			exitWithError(cmd, fmt.Errorf("list supports type %s, got %q", ctxListHelpText(), typ))
		}
		files, err := listContextFiles(t.dir)
		exitWithError(cmd, err)
		agent := getStringFlag(cmd, "agent", false)
		role := getStringFlag(cmd, "role", false)
		if agent != "" || role != "" {
			files, err = filterContextFilesByProvenance(files, agent, role)
			exitWithError(cmd, err)
		}
		outputStringList(cmd, files)
	},
}

// filterContextFilesByProvenance keeps the files whose frontmatter `agent`
// and/or `role` equals the requested value (an empty filter is ignored).
func filterContextFilesByProvenance(files []string, agent, role string) ([]string, error) {
	var out []string
	for _, f := range files {
		data, err := os.ReadFile(f) //#nosec G304 -- path from listContextFiles
		if err != nil {
			return nil, err
		}
		content := string(data)
		if agent != "" && parseFrontmatterField(content, "agent") != agent {
			continue
		}
		if role != "" && parseFrontmatterField(content, "role") != role {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

// ── context task ───────────────────────────────────────────────────────────────
