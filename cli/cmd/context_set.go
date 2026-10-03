package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/frontmatter"
	"github.com/spf13/cobra"
)

// Keys written by set/unset that have a registry or constant meaning.
const (
	ctxKeyAnalysisID = "analysis_id"
	ctxKeyPlanID     = "plan_id"
	ctxDocTitleKey   = "title"
)

// ctxCLIOwnedKeys are frontmatter keys the CLI manages through a dedicated
// command; writing them with set/unset needs --force.
var ctxCLIOwnedKeys = map[string]string{
	ctxFrontmatterUID: "sdt context uid backfill",
	ctxKeyAnalysisID:  "sdt context relations backfill",
	ctxKeyPlanID:      "sdt context relations backfill",
}

var contextSetCmd = &cobra.Command{
	Use:   "set <doc> <key> <value>",
	Short: "Write a top-level frontmatter key with byte fidelity",
	Long: `Write a top-level frontmatter key of a context document. The value is parsed
as a YAML scalar (so 3 is a number, true a bool, [a, b] a list); --raw forces
a string. --append and --remove add or drop one item of a list value.

Only the target key's bytes are replaced: comments, key order, quoting, block
scalars and the body are preserved. A missing key is inserted after its
declared neighbours.

status is validated against the kind vocabulary and updated follows the
per-type rule; uid, analysis_id and plan_id are refused without --force, since
dedicated commands own them.

Examples:
  sdt context set plan/<plan-file> status active
  sdt context set analysis/x.md categories '[new-feature, research]'
  sdt context set notes/x.md tags cli --append
  sdt context set analysis/x.md summary "a quoted: value" --raw`,
	Args: cobra.ExactArgs(3),
	Run:  runContextSet,
}

var contextUnsetCmd = &cobra.Command{
	Use:   "unset <doc> <key>",
	Short: "Remove a top-level frontmatter key",
	Long: `Remove a top-level frontmatter key from a context document, preserving every
other byte. Removing an absent key is a no-op. uid, analysis_id and plan_id are
refused without --force.

Examples:
  sdt context unset analysis/x.md categories`,
	Args: cobra.ExactArgs(2),
	Run:  runContextUnset,
}

func runContextSet(cmd *cobra.Command, args []string) {
	doc, err := resolveContextDoc(cmd, args[:1])
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	key, value := args[1], args[2]
	if err := checkSetKey(cmd, key); err != nil {
		exitWithError(cmd, err)
		return
	}

	//#nosec G304 -- resolved context document
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	content := string(data)
	block, err := frontmatter.Parse(content)
	if err != nil {
		exitWithError(cmd, err)
		return
	}

	rendered, err := renderSetValue(cmd, block, key, value)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	if err := validateSetValue(doc, key, rendered); err != nil {
		exitWithError(cmd, err)
		return
	}

	newYAML, err := block.Set(key, rendered, ctxKeyOrder(doc.Type.kind))
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	doc0 := block.ReplaceBody(content, newYAML)
	doc0 = refreshUpdated(cmd, doc, key, doc0)
	if err := writeContextDocFile(doc.Path, doc0); err != nil {
		exitWithError(cmd, err)
		return
	}
	outputString(cmd, fmt.Sprintf("%s: %s\n", key, rendered))
}

func runContextUnset(cmd *cobra.Command, args []string) {
	doc, err := resolveContextDoc(cmd, args[:1])
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	key := args[1]
	if err := checkSetKey(cmd, key); err != nil {
		exitWithError(cmd, err)
		return
	}
	//#nosec G304 -- resolved context document
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	content := string(data)
	block, err := frontmatter.Parse(content)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	newYAML, err := block.Unset(key)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	doc0 := refreshUpdated(cmd, doc, key, block.ReplaceBody(content, newYAML))
	if err := writeContextDocFile(doc.Path, doc0); err != nil {
		exitWithError(cmd, err)
		return
	}
	outputString(cmd, fmt.Sprintf("removed %s\n", key))
}

// checkSetKey rejects CLI-owned keys (unless --force) and nested keys.
func checkSetKey(cmd *cobra.Command, key string) error {
	if strings.Contains(key, ".") {
		return fmt.Errorf("nested key %q is not supported: only top-level frontmatter keys are addressable", key)
	}
	if hint, owned := ctxCLIOwnedKeys[key]; owned && !getBoolFlag(cmd, "force", false) {
		return fmt.Errorf("%q is managed by the CLI: use %q (or pass --force)", key, hint)
	}
	return nil
}

// renderSetValue turns the value argument into a YAML scalar, honouring --raw
// and the list verbs --append/--remove.
func renderSetValue(cmd *cobra.Command, block *frontmatter.Block, key, value string) (string, error) {
	appendFlag := getBoolFlag(cmd, "append", false)
	removeFlag := getBoolFlag(cmd, "remove", false)
	if appendFlag && removeFlag {
		return "", fmt.Errorf("--append and --remove are mutually exclusive")
	}
	if appendFlag || removeFlag {
		items := parseInlineList(block.RawValue(key))
		quoted := frontmatter.Scalar(value)
		if appendFlag {
			items = append(items, quoted)
		} else {
			items = removeListItem(items, quoted)
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	}
	if getBoolFlag(cmd, "raw", false) {
		return frontmatter.Scalar(value), nil
	}
	// A valid YAML scalar/collection is kept verbatim; else it becomes a string.
	if isYAMLValue(value) {
		return value, nil
	}
	return frontmatter.Scalar(value), nil
}

// validateSetValue checks a written value against the type registry: status
// must be in the kind's vocabulary, and updated must be a valid timestamp.
func validateSetValue(doc ctxResolvedDoc, key, rendered string) error {
	switch key {
	case ctxMapStatus:
		raw := strings.Trim(rendered, `"`)
		if !ctxStatusInVocab(doc.Type, raw) {
			return fmt.Errorf("invalid status %q for type %s (use %s)", raw, doc.Type.kind, ctxStatusVocab(doc.Type))
		}
	case statusUpdated:
		raw := strings.Trim(rendered, `"`)
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			return fmt.Errorf("updated must be an RFC3339 timestamp: %w", err)
		}
	}
	return nil
}

// refreshUpdated rewrites `updated` to now unless the caller set it directly or
// the kind does not carry an `updated` field.
func refreshUpdated(cmd *cobra.Command, doc ctxResolvedDoc, key, content string) string {
	if key == statusUpdated || !ctxHasUpdatedFor(doc.Type.kind) {
		return content
	}
	block, err := frontmatter.Parse(content)
	if err != nil {
		return content
	}
	now := time.Now().UTC().Format(time.RFC3339)
	newYAML, err := block.Set(statusUpdated, now, ctxKeyOrder(doc.Type.kind))
	if err != nil {
		return content
	}
	return block.ReplaceBody(content, newYAML)
}

// ctxKeyOrder returns the ordered neighbour keys used to place a new key after
// its declared siblings, mirroring the per-type frontmatter field order.
func ctxKeyOrder(_ string) []string {
	return ctxFrontmatterOrder
}

// ctxFrontmatterOrder is the declared neighbour order used to place a new key
// after its siblings.
//
//nolint:goconst // the neighbour order is inherently a list of field names
var ctxFrontmatterOrder = []string{
	"kind", ctxFrontmatterUID, "number", ctxDocTitleKey, "summary", "objective",
	"topics", "entities", "categories", ctxMapStatus, "created", statusUpdated,
	"links", "sources",
}

// isYAMLValue reports whether s parses as a non-string YAML scalar or a
// collection, so it can be written verbatim.
func isYAMLValue(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '[', '{':
		return true
	}
	switch strings.ToLower(trimmed) {
	case "true", "false", "null", "~":
		return true
	}
	return isNumeric(trimmed)
}

// isNumeric reports whether s looks like a plain number.
func isNumeric(s string) bool {
	dot := false
	for i, r := range s {
		if (r == '-' || r == '+') && i == 0 {
			continue
		}
		if r == '.' {
			if dot {
				return false
			}
			dot = true
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// parseInlineList parses a flow list "[a, b]" into its raw items (already
// appearing as YAML). A non-list value becomes a one-element list.
func parseInlineList(s string) []string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		if inner == "" {
			return nil
		}
		parts := strings.Split(inner, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if v := strings.TrimSpace(p); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

func removeListItem(items []string, needle string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it) != strings.TrimSpace(needle) {
			out = append(out, it)
		}
	}
	return out
}

func writeContextDocFile(path, content string) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil { //#nosec G306 G703 -- preserves the file's own mode
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func init() {
	addContextStatusRefFlags(contextSetCmd)
	contextSetCmd.Flags().Bool("raw", false, "Force the value to be written as a string")
	contextSetCmd.Flags().Bool("append", false, "Append the value to a list key")
	contextSetCmd.Flags().Bool("remove", false, "Remove the value from a list key")
	contextSetCmd.Flags().Bool("force", false, "Write a CLI-owned key (uid, analysis_id, plan_id)")
	addContextStatusRefFlags(contextUnsetCmd)
	contextUnsetCmd.Flags().Bool("force", false, "Remove a CLI-owned key (uid, analysis_id, plan_id)")
	contextCmd.AddCommand(contextSetCmd, contextUnsetCmd)
}
