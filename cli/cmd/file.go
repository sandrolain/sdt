//go:build !wasm

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/sandrolain/sdt/internal/mdstruct"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// from: https://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-exists
func fileExists(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

var fileReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read a file or a line range of it",
	Long: `Print a file, optionally only a line range.

WHEN — you need the content of a file, or a slice of it, and have no host
reader.

WHEN NOT — you need a document from the corpus by what it is: ` + "`sdt context show <path>`" + `
prints a ` + "`context/`" + ` work file, and ` + "`--section`" + ` selects one of its sections without
counting lines.

RETURNS — the file content, or lines ` + "`--lines <from>:<to>`" + ` of it, 1-based and inclusive. A
range that starts past the last line, or that is inverted, is an error rather
than an empty answer; an upper bound past the last line clamps to the end.

` + "`--count`" + ` reports the number of lines instead of them — the ` + "`wc -l`" + ` fallback. It
composes with ` + "`--lines`" + `, counting the selection rather than contradicting it, and a
file with no lines has a count of 0.

` + "`--numbered`" + ` prefixes each printed line with its 1-based number in the file, the
shape a host reader prints, so a citation can be quoted verbatim. ` + "`--lines -N:`" + `
selects the last N lines — ` + "`tail -n`" + ` without a second verb.

Earlier versions ignored ` + "`--format json`" + ` here and printed raw text: a global flag
that silently did nothing is the defect the change fixed. The JSON answer is an
object ` + "`{path, lines, count, numbered}`" + `.

EXAMPLE — ` + "`sdt read -p cli/cmd/file.go --lines 31:49`" + `
`,
	Example: `  sdt read -p go.mod
  sdt read -p cli/cmd/context_search.go --lines 168:214
  sdt read -p go.mod --count`,
	Run: func(cmd *cobra.Command, args []string) {
		file := getStringFlag(cmd, "file", true)

		// A corpus reference (`plan/<file>`, `context/analysis/<file>`, a bare
		// slug) resolves to its document: one reader for the working tree and the
		// corpus, instead of a second `context show` verb.
		if resolved, ok := resolveCorpusReadRef(file); ok {
			file = resolved
		}

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}

		// #nosec G304 -- implementation of generic utility to read file from CLI flag
		content, err := os.ReadFile(file)
		exitWithError(cmd, err)
		lines := getStringFlag(cmd, "lines", false)
		section := getStringFlag(cmd, "section", false)
		numbered := getBoolFlag(cmd, "numbered", false)
		if getBoolFlag(cmd, "count", false) {
			selected, _, err := selectLinesFrom(string(content), lines)
			exitWithError(cmd, err)
			outputLineCount(cmd, len(selected))
			return
		}
		if section != "" {
			if numbered {
				exitWithError(cmd, fmt.Errorf("--numbered numbers the whole file: narrow with --lines instead of --section"))
			}
			content = []byte(readSectionOrExit(cmd, string(content), section))
			lines = ""
		}
		selected, start, err := selectLinesFrom(string(content), lines)
		exitWithError(cmd, err)
		if numbered {
			selected = numberLines(selected, start)
		}
		switch getFormat(cmd) {
		case fmtJSON:
			payload := struct {
				Path     string   `json:"path"`
				Lines    []string `json:"lines"`
				Count    int      `json:"count"`
				Numbered bool     `json:"numbered"`
			}{Path: file, Lines: selected, Count: len(selected), Numbered: numbered}
			out, err := json.MarshalIndent(payload, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, append(out, '\n'))
		case fmtYAML:
			payload := struct {
				Path     string   `yaml:"path"`
				Lines    []string `yaml:"lines"`
				Count    int      `yaml:"count"`
				Numbered bool     `yaml:"numbered"`
			}{Path: file, Lines: selected, Count: len(selected), Numbered: numbered}
			out, err := yaml.Marshal(payload)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			if lines != "" || section != "" || numbered {
				outputString(cmd, strings.Join(selected, "\n")+"\n")
				return
			}
			outputBytes(cmd, content)
		}
	},
}

// resolveCorpusReadRef maps a corpus reference to its document path. It returns
// false for anything that does not resolve, so a plain file path is read as
// written and a typo still fails with the file's own error.
func resolveCorpusReadRef(ref string) (string, bool) {
	if ref == "" {
		return "", false
	}
	doc, err := resolveContextDocPath(ref)
	if err != nil {
		return "", false
	}
	return doc.Path, true
}

// readSectionOrExit prints one section of a document, matched by id or heading.
// It is shared by `sdt read --section` and the deprecated `context show`, so the
// two cannot disagree.
func readSectionOrExit(cmd *cobra.Command, content, section string) string {
	body := ctxDocBody(content)
	for _, s := range mdstruct.SplitSections(body) {
		if strings.EqualFold(s.ID, section) || strings.EqualFold(s.Heading, section) {
			return strings.TrimRight(s.Body, "\n") + "\n"
		}
	}
	exitWithError(cmd, fmt.Errorf("section %q not found", section))
	return ""
}

var fileWriteCmd = &cobra.Command{
	Use:   "write",
	Short: "File Write",
	Long:  `File Write`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)

		files := getStringFlag(cmd, "file", true)
		overwrite := getBoolFlag(cmd, "overwrite", false)
		multi := getBoolFlag(cmd, "multi", false)
		binary := getBoolFlag(cmd, "binary", false)

		var contents [][]byte

		if multi {
			var data []string
			exitWithError(cmd, json.Unmarshal(byt, &data))
			contents = make([][]byte, len(data))
			for i, str := range data {
				if binary {
					content, err := base64.StdEncoding.DecodeString(str)
					exitWithError(cmd, err)
					contents[i] = content
				} else {
					contents[i] = []byte(str)
				}
			}
		} else {
			contents = make([][]byte, 1)
			if binary {
				cnt, err := base64.StdEncoding.DecodeString(string(byt))
				exitWithError(cmd, err)
				contents[0] = cnt
			} else {
				contents[0] = byt
			}
		}

		paths := strings.Split(files, ",")

		res := make([]string, len(paths))

		for i, path := range paths {
			path, err := filepath.Abs(path)
			exitWithError(cmd, err)
			exist, err := fileExists(path)
			exitWithError(cmd, err)
			if exist && !overwrite {
				exitWithError(cmd, fmt.Errorf(`file "%s" already exist`, path))
			}

			byt := contents[i]
			exitWithError(cmd, os.WriteFile(path, byt, 0600))

			res[i] = path
		}

		outputString(cmd, strings.Join(res, "\n"))
	},
}

func init() {
	pf := fileReadCmd.PersistentFlags()
	pf.StringP("file", "p", "", "File path")
	pf.String("lines", "", "Print only this line range (from:to, 1-based)")
	pf.String("section", "", "Print only this section (by id or heading)")
	pf.Bool("count", false, "Print the number of lines instead of them (with --lines: of the range)")
	pf.BoolP("numbered", "n", false, "Prefix each line with its 1-based number")

	pf = fileWriteCmd.PersistentFlags()
	pf.StringP("file", "p", "", "File path")
	pf.BoolP("multi", "m", false, "Input as JSON array with multiple contents")
	pf.BoolP("overwrite", "o", false, "Overwrite if already exist")
	pf.BoolP("binary", "b", false, "Input as Base64 encoded content")

	rootCmd.AddCommand(fileReadCmd)
	rootCmd.AddCommand(fileWriteCmd)
}
