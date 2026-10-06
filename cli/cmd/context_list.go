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
			terms = append(terms, ctxquery.Term{Key: ctxFrontmatterAgent, Value: agent})
		}
		if role := getStringFlag(cmd, "role", false); role != "" {
			terms = append(terms, ctxquery.Term{Key: "role", Value: role})
		}
		var statuses []string
		if status := getStringFlag(cmd, "status", false); status != "" {
			// The kind is resolved, so --status is checked against that kind's
			// closed vocabulary instead of silently matching nothing. A kind with
			// no status field (worklog/notes/tmp) is an explicit error, not an
			// empty list.
			if len(t.statuses) == 0 {
				exitWithError(cmd, fmt.Errorf("--status is not supported for --type %s (no status vocabulary)", typ))
			}
			if !ctxStatusInVocab(t, status) {
				exitWithError(cmd, fmt.Errorf("invalid status %q for type %s (use %s)", status, typ, ctxStatusVocab(t)))
			}
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
