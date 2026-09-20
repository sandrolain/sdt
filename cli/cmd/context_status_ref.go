package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ── context document resolution (status get/set, rename, archive) ──────────────

// ctxResolvedDoc is an existing context/ work file with its registry type.

type ctxResolvedDoc struct {
	Path string
	Type ctxDocType
}

// resolveContextDoc accepts a path under context/ (absolute or relative, with
// or without the .md suffix) or identity flags (--type/--slug plus
// --number/--phase/--plan) and returns the existing file plus its type.

func resolveContextDoc(cmd *cobra.Command, args []string) (ctxResolvedDoc, error) {
	if len(args) > 0 {
		return resolveContextDocPath(args[0])
	}
	return resolveContextDocIdentity(cmd)
}

// ctxCwd returns the current working directory used as the base for resolving
// absolute document paths.
func ctxCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func resolveContextDocPath(ref string) (ctxResolvedDoc, error) {
	ref = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(ref), sdtMarkdownExt))
	if ref == "" {
		return ctxResolvedDoc{}, errors.New("document reference is required (a path or --type/--slug identity)")
	}

	var candidate string
	switch {
	case filepath.IsAbs(ref):
		rel, err := filepath.Rel(ctxCwd(), ref)
		if err != nil {
			return ctxResolvedDoc{}, err
		}
		candidate = filepath.Clean(rel)
	case strings.HasPrefix(ref, sdtWorkDir+"/"):
		candidate = filepath.Clean(ref)
	default:
		// accept "plan/foo" as a shorthand for "context/plan/foo"
		candidate = filepath.Clean(filepath.Join(sdtWorkDir, ref))
	}

	if candidate == sdtWorkDir || !strings.HasPrefix(candidate, sdtWorkDir+string(filepath.Separator)) {
		return ctxResolvedDoc{}, fmt.Errorf("path %q is outside %s/", ref, sdtWorkDir)
	}

	// tmp entries have no .md suffix; every other work file is markdown.
	if filepath.Ext(candidate) == "" && ctxFirstSegment(candidate) != ctxTypeTmp {
		candidate += sdtMarkdownExt
	}

	t, ok := ctxTypeForDir(ctxFirstSegmentDir(candidate))
	if !ok {
		return ctxResolvedDoc{}, fmt.Errorf("unknown context type for path %q", candidate)
	}

	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return ctxResolvedDoc{}, fmt.Errorf("no such document: %s", candidate)
		}
		return ctxResolvedDoc{}, err
	}
	if info.IsDir() {
		return ctxResolvedDoc{}, fmt.Errorf("%s is a directory", candidate)
	}
	return ctxResolvedDoc{Path: candidate, Type: t}, nil
}

// ctxFirstSegment returns the directory name right under context/ for a
// context-relative path ("context/plan/foo.md" -> "plan").
func ctxFirstSegment(path string) string {
	rel := strings.TrimPrefix(path, sdtWorkDir+string(filepath.Separator))
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	return parts[0]
}

// ctxFirstSegmentDir returns the full dir ("context/plan") for the first
// segment under context/.
func ctxFirstSegmentDir(path string) string {
	return filepath.Join(sdtWorkDir, ctxFirstSegment(path))
}

func resolveContextDocIdentity(cmd *cobra.Command) (ctxResolvedDoc, error) {
	typ := getStringFlag(cmd, "type", false)
	if typ == ctxDocAliasDecision {
		typ = ctxTypeDecision
	}
	if typ == "" {
		return ctxResolvedDoc{}, errors.New("document reference is required (a path or --type/--slug identity)")
	}
	t, ok := ctxTypeLookup(typ)
	if !ok {
		return ctxResolvedDoc{}, fmt.Errorf("unknown type %q (use %s)", typ, ctxTypeHelpText(ctxPathTypes()))
	}
	slug := sanitizeSlug(getStringFlag(cmd, "slug", false))

	var path string
	switch t.scheme {
	case ctxSchemeDecision:
		number := getStringFlag(cmd, "number", false)
		if number == "" {
			return ctxResolvedDoc{}, errors.New("--number is required for type decision")
		}
		if err := validateDecisionNumber(number); err != nil {
			return ctxResolvedDoc{}, err
		}
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type decision")
		}
		path = filepath.Join(t.dir, number+"-"+slug+sdtMarkdownExt)
	case ctxSchemePhase:
		if !t.pathSupported {
			return ctxResolvedDoc{}, fmt.Errorf("type %q cannot be addressed by identity", typ)
		}
		phase, plan, err := taskTarget(cmd)
		if err != nil {
			return ctxResolvedDoc{}, err
		}
		path = taskFileFor(phase, plan)
	case ctxSchemeTmpBySlug:
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type tmp")
		}
		path = filepath.Join(t.dir, slug)
	case ctxSchemeDated:
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type " + typ)
		}
		matches, err := filepath.Glob(filepath.Join(t.dir, "*-"+slug+sdtMarkdownExt))
		if err != nil {
			return ctxResolvedDoc{}, err
		}
		switch len(matches) {
		case 0:
			return ctxResolvedDoc{}, fmt.Errorf("no %s document with slug %q under %s/", typ, slug, t.dir)
		case 1:
			path = matches[0]
		default:
			return ctxResolvedDoc{}, fmt.Errorf("multiple %s documents match slug %q (%v); pass a full path instead", typ, slug, matches)
		}
	case ctxSchemeBare, ctxSchemeSubpath:
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type " + typ)
		}
		path = filepath.Join(t.dir, slug+sdtMarkdownExt)
	default:
		return ctxResolvedDoc{}, fmt.Errorf("type %q cannot be addressed by identity", typ)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ctxResolvedDoc{}, fmt.Errorf("no such document: %s", path)
		}
		return ctxResolvedDoc{}, err
	}
	if info.IsDir() {
		return ctxResolvedDoc{}, fmt.Errorf("%s is a directory", path)
	}
	return ctxResolvedDoc{Path: path, Type: t}, nil
}

// ctxStatusBearing reports whether a type carries a status field at all
// (worklog/notes/tmp do not).

func ctxStatusBearing(t ctxDocType) bool {
	return len(t.statuses) > 0
}

// ── context status get ─────────────────────────────────────────────────────────

type ctxStatusResult struct {
	Path    string `json:"path" yaml:"path"`
	Type    string `json:"type" yaml:"type"`
	Status  string `json:"status" yaml:"status"`
	Updated string `json:"updated,omitempty" yaml:"updated,omitempty"`
}

func outputContextStatus(cmd *cobra.Command, res ctxStatusResult) {
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
		if res.Status == "" {
			res.Status = "(none)"
		}
		outputString(cmd, res.Status+"\n")
	}
}

var contextStatusGetCmd = &cobra.Command{
	Use:   "get <ref>",
	Short: "Print the frontmatter status of a context/ document",
	Long: `Print the frontmatter status field of an existing context/ document,
resolved by path (absolute or relative, with or without the .md suffix) or by
identity (--type/--slug, plus --number for decision or --phase/--plan for task
files).

Types without a status field (worklog, notes, tmp) are rejected.

Examples:
  sdt context status get context/plan/20260920-131900-plan-x.md
  sdt context status get plan/20260920-131900-plan-x
  sdt context status get --type decision --number 0001 --slug auth-choice
  sdt context status get --type analysis --slug backend --format json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		if !ctxStatusBearing(doc.Type) {
			exitWithError(cmd, fmt.Errorf("type %s does not carry a status field (status get unsupported)", ctxKindLabel(doc.Type)))
		}
		data, err := os.ReadFile(doc.Path) //#nosec G304 -- user work file
		exitWithError(cmd, err)
		outputContextStatus(cmd, ctxStatusResult{
			Path:    doc.Path,
			Type:    doc.Type.kind,
			Status:  frontmatterField(string(data), ctxMapStatus),
			Updated: frontmatterField(string(data), "updated"),
		})
	},
}

// ── context status set ─────────────────────────────────────────────────────────

// frontmatterPatch describes a frontmatter key substitution. Patches applied
// in order; absent keys are appended before the closing `---`.

type frontmatterPatch struct {
	key   string
	value string
}

// setFrontmatterFields rewrites the frontmatter keys of content per patches,
// appending missing keys before the closing delimiter. It returns the new
// content and whether anything changed (contents without a frontmatter block
// are returned untouched).

func setFrontmatterFields(content string, patches []frontmatterPatch) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return content, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == ctxFrontmatterDelim {
			end = i
			break
		}
	}
	if end < 0 {
		return content, false
	}

	changed := false
	present := map[string]bool{}
	for i := 1; i < end; i++ {
		key := strings.SplitN(lines[i], ":", 2)[0]
		if key == "" {
			continue
		}
		present[key] = true
		for _, p := range patches {
			if p.key == key {
				if strings.TrimSpace(strings.TrimPrefix(lines[i], key+":")) != p.value {
					lines[i] = key + ": " + p.value
					changed = true
				}
			}
		}
	}

	var missing []string
	for _, p := range patches {
		if !present[p.key] {
			missing = append(missing, p.key+": "+p.value)
		}
	}
	if len(missing) > 0 {
		var tail []string
		tail = append(tail, lines[:end]...)
		tail = append(tail, missing...)
		tail = append(tail, lines[end:]...)
		lines = tail
		changed = true
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

// ctxStatusSetPatches builds the frontmatter patches for a status transition:
// the new status always; the refreshed `updated` timestamp when the kind's
// contract requires it (decisions never get an updated refresh).

func ctxStatusSetPatches(t ctxDocType, status string) []frontmatterPatch {
	patches := []frontmatterPatch{{key: ctxMapStatus, value: status}}
	if t.hasUpdated {
		patches = append(patches, frontmatterPatch{key: "updated", value: contextNow().UTC().Format(time.RFC3339)})
	}
	return patches
}

var contextStatusSetCmd = &cobra.Command{
	Use:   "set <ref> --status <v>",
	Short: "Set the frontmatter status of a context/ document",
	Long: `Set the frontmatter status field of an existing context/ document,
resolved by path or identity like ` + "`status get`" + `. The value is validated
against the kind's status vocabulary (from the instruction contracts). When
the kind requires ` + "`updated`" + ` it is refreshed to now.

Types without a status field (worklog, notes, tmp) are rejected. Decisions are
append-only: only the status field changes, never the body or filename.

Examples:
  sdt context status set context/analysis/20260920-130000-x.md --status archived
  sdt context status set --type decision --number 0001 --slug auth-choice --status accepted
  sdt context status set --type proposal --slug provenance --status review`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		status := getStringFlag(cmd, "status", true)
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		if !ctxStatusBearing(doc.Type) {
			exitWithError(cmd, fmt.Errorf("type %s does not carry a status field (status set unsupported)", ctxKindLabel(doc.Type)))
		}
		if !ctxStatusInVocab(doc.Type, status) {
			exitWithError(cmd, fmt.Errorf("invalid status %q for type %s (use %s)", status, doc.Type.kind, ctxStatusVocab(doc.Type)))
		}
		data, err := os.ReadFile(doc.Path) //#nosec G304 -- user work file
		exitWithError(cmd, err)
		content, changed := setFrontmatterFields(string(data), ctxStatusSetPatches(doc.Type, status))
		if !changed {
			exitWithError(cmd, fmt.Errorf("no status field written to %s (missing frontmatter)", doc.Path))
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(doc.Path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}

		updated := ""
		if doc.Type.hasUpdated {
			updated = contextNow().UTC().Format(time.RFC3339)
		}
		switch getFormat(cmd) {
		case fmtJSON, fmtYAML:
			res := ctxStatusResult{Path: doc.Path, Type: doc.Type.kind, Status: status, Updated: updated}
			if getFormat(cmd) == fmtJSON {
				out, err := json.MarshalIndent(res, "", "  ")
				exitWithError(cmd, err)
				outputBytes(cmd, out)
			} else {
				out, err := yaml.Marshal(res)
				exitWithError(cmd, err)
				outputBytes(cmd, out)
			}
		default:
			outputString(cmd, "ok\n")
		}
	},
}

// ctxStatusInVocab reports whether v belongs to the kind's status vocabulary.

func ctxStatusInVocab(t ctxDocType, v string) bool {
	for _, s := range t.statuses {
		if s == v {
			return true
		}
	}
	return false
}

// ctxStatusVocab renders the pipe-joined vocabulary for error messages.

func ctxStatusVocab(t ctxDocType) string {
	return strings.Join(t.statuses, "|")
}

func addContextStatusRefFlags(c *cobra.Command) {
	c.Flags().String("type", "", "Document type for identity resolution")
	c.Flags().String("slug", "", "Slug for identity resolution")
	c.Flags().String("number", "", "Decision number (type decision)")
	c.Flags().String("phase", "", "Phase for a task file (type tasks)")
	c.Flags().String("plan", "", "Plan reference for a task file (type tasks)")
}

func init() {
	addContextStatusRefFlags(contextStatusGetCmd)
	addContextStatusRefFlags(contextStatusSetCmd)
	contextStatusSetCmd.Flags().String("status", "", "New status value (validated against the kind vocabulary)")
	contextStatusCmd.AddCommand(contextStatusGetCmd, contextStatusSetCmd)
}
