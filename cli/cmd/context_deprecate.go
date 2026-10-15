//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxDeprecateResult is the reported outcome of one retirement: the transition
// the call made, and the successor it recorded when one was named.
type ctxDeprecateResult struct {
	Path         string `json:"path" yaml:"path"`
	Kind         string `json:"kind" yaml:"kind"`
	From         string `json:"from,omitempty" yaml:"from,omitempty"`
	To           string `json:"to" yaml:"to"`
	SupersededBy string `json:"supersededBy,omitempty" yaml:"supersededBy,omitempty"`
	DryRun       bool   `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`
}

// ctxRetiredRefusal explains why a type cannot be retired. It distinguishes the
// two cases because they are different sentences: a type with no status field at
// all is history, and one whose vocabulary simply carries no retirement has to be
// told what it does carry instead.
func ctxRetiredRefusal(t ctxDocType) error {
	if len(t.statuses) == 0 {
		return fmt.Errorf("type %s carries no status field, so it has no retired state to set", ctxKindLabel(t))
	}
	return fmt.Errorf("type %s carries no retired status: its vocabulary is %s, and none of those values means \"kept, no longer current\" — choose one with `sdt context status set`", ctxKindLabel(t), ctxStatusVocab(t))
}

// ctxDocStatus reads a document's current status; "" when the field is absent.
func ctxDocStatus(path string) (string, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- a resolved context/ work file
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(parseFrontmatterField(string(data), ctxMapStatus)), nil
}

// ctxCorpusRef is the form a reference takes in frontmatter: corpus-relative,
// with the extension, and without the `context/` prefix every on-disk path
// carries.
func ctxCorpusRef(path string) string {
	return strings.TrimPrefix(normalizeContextRef(path), sdtWorkDir+"/")
}

func outputDeprecateResult(cmd *cobra.Command, res ctxDeprecateResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		line := fmt.Sprintf("%s: %s → %s", res.Path, res.From, res.To)
		if res.SupersededBy != "" {
			line += " (superseded by " + res.SupersededBy + ")"
		}
		if res.DryRun {
			line = "would retire " + strings.TrimPrefix(line, res.Path+": ")
		}
		outputString(cmd, line+"\n")
	}
}

var contextDeprecateCmd = &cobra.Command{
	Use:   "deprecate <ref>",
	Short: "Retire a document by changing its status, without moving it",
	Long: `Retire a document: set the retired status its own type's vocabulary carries,
and nothing else. The file does not move, so every reference to it keeps
resolving and a reader answers "is this still valid" from the state rather than
from where the document sits.

WHEN — a document is still worth reading and no longer current: an analysis a
later analysis replaced, a decision no longer in force, a prompt nobody should
reach for again. Pass ` + "`--by <ref>`" + ` to name the successor when one document
replaced another; the value is recorded in ` + "`superseded_by`" + ` and must resolve,
because a dangling successor is worse than none.

WHEN NOT — the file itself must go: ` + "`deprecate`" + ` keeps it, by design. A type
whose vocabulary carries no retired value (` + "`questions`" + `, ` + "`commands`" + `) is
refused by name rather than given another type's word, and a type with no status
field at all (worklog, notes) has no retired state to set.

RETURNS — the path, the type, the status it had and the one it now has, plus the
successor when one was named. The retired value per type is declared in the type
registry, so the answer here and the value ` + "`sdt context vocab`" + ` prints cannot
drift apart.

EXAMPLE — ` + "`sdt context deprecate analysis/20260101-000000-old --by analysis/20260202-000000-new`" + `
          ` + "`sdt context deprecate --type prompt --slug legacy --dry-run`" + ``,
	Example: `  sdt context deprecate analysis/20260101-000000-old
  sdt context deprecate analysis/20260101-000000-old --by analysis/20260202-000000-new
  sdt context deprecate --type decision --number 0007 --slug auth-choice`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		if doc.Type.retiredStatus == "" {
			exitWithError(cmd, ctxRetiredRefusal(doc.Type))
			return
		}
		retired := doc.Type.retiredStatus

		by := strings.TrimSpace(getStringFlag(cmd, "by", false))
		byRef := ""
		if by != "" {
			resolved, ok := ctxResolveCorpusRef(by)
			if !ok {
				exitWithError(cmd, fmt.Errorf("--by %s does not resolve to a document under %s (use a corpus-relative reference, as `links` stores it)", by, sdtWorkDir))
				return
			}
			if resolved == doc.Path {
				exitWithError(cmd, fmt.Errorf("--by %s is the document being retired", by))
				return
			}
			byRef = ctxCorpusRef(resolved)
		}

		current, err := ctxDocStatus(doc.Path)
		exitWithError(cmd, err)
		if current == retired {
			exitWithError(cmd, fmt.Errorf("%s is already %s", doc.Path, retired))
			return
		}

		res := ctxDeprecateResult{Path: doc.Path, Kind: doc.Type.kind, From: current, To: retired, SupersededBy: byRef}
		if getBoolFlag(cmd, "dry-run", false) {
			res.DryRun = true
			outputDeprecateResult(cmd, res)
			return
		}

		patches := ctxStatusSetPatches(doc.Type, retired)
		if byRef != "" {
			patches = append(patches, frontmatterPatch{key: ctxFrontmatterSupersededBy, value: byRef})
		}
		previous, err := ctxApplyFrontmatter(cmd, doc, patches)
		exitWithError(cmd, err)
		if previous != "" {
			res.From = previous
		}
		outputDeprecateResult(cmd, res)
		cascadeAfterWrite(cmd, doc.Path)
	},
}

func init() {
	f := contextDeprecateCmd.Flags()
	f.String("by", "", "Reference of the document that supersedes this one")
	f.Bool("dry-run", false, "Report the transition without writing it")
	addContextStatusRefFlags(contextDeprecateCmd)
}
