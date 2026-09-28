package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/ctxquery"
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
  sdt context list --type plan --status active --last 7d
  sdt context list --type analysis --where categories=refactor` + ctxQueryHelp,
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

		var terms []ctxquery.Term
		if agent := getStringFlag(cmd, "agent", false); agent != "" {
			terms = append(terms, ctxquery.Term{Key: "agent", Value: agent})
		}
		if role := getStringFlag(cmd, "role", false); role != "" {
			terms = append(terms, ctxquery.Term{Key: "role", Value: role})
		}
		var statuses []string
		if status := getStringFlag(cmd, "status", false); status != "" {
			statuses = []string{status}
		}
		filter, err := buildQueryFilter(cmd, nil, statuses, getStringArrayFlag(cmd, "category", false), terms)
		exitWithError(cmd, err)
		files, err = filterFilesContext(files, filter)
		exitWithError(cmd, err)
		outputStringList(cmd, files)
	},
}

// ── context task ───────────────────────────────────────────────────────────────
