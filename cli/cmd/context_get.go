package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/spf13/cobra"
)

var contextGetCmd = &cobra.Command{
	Use:   "get <doc> [<key>...]",
	Short: "Read a document's frontmatter: the raw block or named top-level keys",
	Long: `Read the frontmatter of a context document. The document is resolved like
'sdt context status get' (a path under context/, or --type/--slug identity
flags).

With no key, the raw frontmatter block (including its --- fences) is printed
byte-exact. With one or more keys, one value is printed per requested key, in
the order asked.

Only top-level keys are addressable: a dotted/nested key is rejected with a
named error rather than returning an empty value. A missing key prints an
empty line on stdout, a note on stderr, and exits 1. Under --format json|yaml
the result is a mapping, so a missing key is an explicit null.

Examples:
  sdt context get plan/<plan-file>        # the raw block
  sdt context get plan/<plan-file> status # one value
  sdt context get --type analysis --slug x objective title`,
	Args: cobra.MinimumNArgs(0),
	Run:  runContextGet,
}

func runContextGet(cmd *cobra.Command, args []string) {
	ref := args
	var keys []string
	if len(args) > 1 {
		ref = args[:1]
		keys = args[1:]
	}
	doc, err := resolveContextDoc(cmd, ref)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	//#nosec G304 -- the path is a resolved context document
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	// Normalize CRLF so a CRLF document reads like an LF one; the shared
	// contextwiki readers otherwise see no frontmatter block at all.
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	fm, _ := contextwiki.SplitFrontmatter(content)
	if fm == "" {
		exitWithError(cmd, fmt.Errorf("%s has no frontmatter block", doc.Path))
		return
	}

	if len(keys) == 0 {
		outputString(cmd, fm)
		return
	}

	for _, key := range keys {
		if strings.Contains(key, ".") {
			exitWithError(cmd, fmt.Errorf("nested key %q is not supported: only top-level frontmatter keys are addressable", key))
			return
		}
	}

	missing := false
	values := make(map[string]*string, len(keys))
	pairs := make([]struct {
		key   string
		value any
	}, 0, len(keys))
	for _, key := range keys {
		if v, ok := contextwiki.FrontmatterValues(content)[key]; ok && len(v) > 0 {
			value := strings.Join(v, ",")
			values[key] = &value
			pairs = append(pairs, struct {
				key   string
				value any
			}{key, value})
			continue
		}
		missing = true
		values[key] = nil
		pairs = append(pairs, struct {
			key   string
			value any
		}{key, nil})
	}

	switch getFormat(cmd) {
	case fmtJSON:
		out := make(map[string]any, len(keys))
		for _, p := range pairs {
			out[p.key] = p.value
		}
		data, err := json.MarshalIndent(out, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(data)+"\n")
	case fmtYAML:
		out := make(map[string]any, len(keys))
		for _, p := range pairs {
			out[p.key] = p.value
		}
		data, err := yaml.Marshal(out)
		exitWithError(cmd, err)
		outputBytes(cmd, data)
	default:
		for _, p := range pairs {
			if p.value == nil {
				outputString(cmd, "\n")
				continue
			}
			outputString(cmd, fmt.Sprintf("%v\n", p.value))
		}
	}

	if missing {
		exitWithError(cmd, fmt.Errorf("one or more requested keys are absent"))
	}
}

func init() {
	addContextStatusRefFlags(contextGetCmd)
	contextCmd.AddCommand(contextGetCmd)
}
