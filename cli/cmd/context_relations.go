package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ── typed, bidirectional parent relations ──────────────────────────────────────
//
// The lifecycle chain is typed explicitly, keyed by the immutable `uid`:
// a plan carries `analysis_id` and the analysis carries the reverse `plans_ids`;
// a task file carries `plan_id` and the plan carries `tasks_ids`. The fields
// complement `sources` (analysis 20260927-135445, decisions Q6/Q7).

// ctxParentField is the child→parent scalar field for a child kind.
func ctxParentField(childKind string) string {
	switch childKind {
	case ctxTypePlan:
		return "analysis_id"
	case ctxTypeTasks:
		return "plan_id"
	}
	return ""
}

// ctxChildrenField is the parent→children reverse list field for a parent kind.
func ctxChildrenField(parentKind string) string {
	switch parentKind {
	case ctxTypeAnalysis:
		return "plans_ids"
	case ctxTypePlan:
		return "tasks_ids"
	}
	return ""
}

// ctxParentKind is the kind a child kind is contained by.
func ctxParentKind(childKind string) string {
	switch childKind {
	case ctxTypePlan:
		return ctxTypeAnalysis
	case ctxTypeTasks:
		return ctxTypePlan
	}
	return ""
}

// ctxDocUID reads the `uid` frontmatter field of a document path.
func ctxDocUID(path string) string {
	data, err := os.ReadFile(path) //#nosec G304 -- context work file
	if err != nil {
		return ""
	}
	return parseFrontmatterField(string(data), ctxFrontmatterUID)
}

// ctxDocKind reads the `kind` frontmatter field of a document path.
func ctxDocKind(path string) string {
	data, err := os.ReadFile(path) //#nosec G304 -- context work file
	if err != nil {
		return ""
	}
	return parseFrontmatterField(string(data), "kind")
}

// ctxSourceParent resolves the first `sources` reference of content to a parent
// of the wanted kind, returning its path and uid. It is the derivation edge the
// typed parent relation mirrors.
func ctxSourceParent(content, wantedKind string) (path, uid string) {
	for _, ref := range parseFrontmatterList(content, ctxFrontmatterSources) {
		abs, ok := ctxResolvePath(sdtWorkDir, ref)
		if !ok {
			continue
		}
		if ctxDocKind(abs) != wantedKind {
			continue
		}
		if u := ctxDocUID(abs); u != "" {
			return abs, u
		}
	}
	return "", ""
}

// appendFrontmatterListValue appends value to a YAML block-list frontmatter
// field, creating the field when absent. It is idempotent: an already-present
// value leaves the content untouched.
func appendFrontmatterListValue(content, field, value string) (string, bool) {
	for _, v := range parseFrontmatterList(content, field) {
		if strings.TrimSpace(v) == value {
			return content, false
		}
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return content, false
	}
	end, fieldIdx := -1, -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == ctxFrontmatterDelim {
			end = i
			break
		}
		if strings.TrimSpace(strings.SplitN(lines[i], ":", 2)[0]) == field {
			fieldIdx = i
		}
	}
	if end < 0 {
		return content, false
	}
	if fieldIdx < 0 {
		out := append([]string{}, lines[:end]...)
		out = append(out, field+":", "  - "+value)
		out = append(out, lines[end:]...)
		return strings.Join(out, "\n"), true
	}
	insertAt := fieldIdx + 1
	for insertAt < end && strings.HasPrefix(strings.TrimSpace(lines[insertAt]), "- ") {
		insertAt++
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, "  - "+value)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n"), true
}

// linkChildToParent appends childUID to the parent's reverse list field
// (idempotently), keeping the child→parent edge mirrored on the parent.
func linkChildToParent(parentPath, childUID, field string) error {
	if parentPath == "" || childUID == "" || field == "" {
		return nil
	}
	data, err := os.ReadFile(parentPath) //#nosec G304,G703 -- context work file
	if err != nil {
		return err
	}
	updated, changed := appendFrontmatterListValue(string(data), field, childUID)
	if !changed {
		return nil
	}
	return writeWorkFile(parentPath, updated)
}

// stampChildParent injects the typed parent scalar on a child document content
// from the parent uid. Returns the content unchanged when either id is missing.
func stampChildParent(content, childKind, parentUID string) (string, bool) {
	field := ctxParentField(childKind)
	childUID := parseFrontmatterField(content, ctxFrontmatterUID)
	if field == "" || parentUID == "" || childUID == "" {
		return content, false
	}
	return setFrontmatterFields(content, []frontmatterPatch{{key: field, value: parentUID}})
}

// ── bidirectional-consistency lint ─────────────────────────────────────────────

type ctxRelDoc struct {
	path       string
	kind       string
	uid        string
	analysisID string
	planID     string
	plansIDs   []string
	tasksIDs   []string
	content    string
}

// lintParentRelations validates the typed parent relations corpus-wide: a
// missing typed parent where a resolvable `sources` parent exists is a
// SUGGESTION (adoption; the relation backfill resolves it), while an
// unresolvable id, a wrong parent kind, or a one-sided edge is a WARNING
// (drift). Uses the immutable uid as the key.
func lintParentRelations(files []string) []ctxLintIssue {
	docs := map[string]ctxRelDoc{}
	var all []ctxRelDoc
	for _, path := range files {
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		doc := ctxRelDoc{
			path:       path,
			kind:       parseFrontmatterField(content, "kind"),
			uid:        parseFrontmatterField(content, ctxFrontmatterUID),
			analysisID: parseFrontmatterField(content, "analysis_id"),
			planID:     parseFrontmatterField(content, "plan_id"),
			plansIDs:   parseFrontmatterList(content, "plans_ids"),
			tasksIDs:   parseFrontmatterList(content, "tasks_ids"),
			content:    content,
		}
		all = append(all, doc)
		if doc.uid != "" {
			docs[doc.uid] = doc
		}
	}

	var issues []ctxLintIssue
	for _, doc := range all {
		switch doc.kind {
		case ctxTypePlan:
			if doc.analysisID != "" {
				issues = append(issues, checkChildRelation(docs, doc, doc.analysisID, ctxTypeAnalysis, "analysis_id", "plans_ids")...)
			} else if _, uid := ctxSourceParent(doc.content, ctxTypeAnalysis); uid != "" {
				issues = append(issues, ctxLintIssue{Path: doc.path, Priority: ctxLintSuggestion, Message: "plan has no `analysis_id` for its sourced analysis (run `sdt context relations backfill`)"})
			}
			issues = append(issues, checkParentListRelation(docs, doc, doc.tasksIDs, ctxTypeTasks, "plan_id", "tasks_ids")...)
		case ctxTypeTasks:
			if doc.planID != "" {
				issues = append(issues, checkChildRelation(docs, doc, doc.planID, ctxTypePlan, "plan_id", "tasks_ids")...)
			} else if _, uid := ctxSourceParent(doc.content, ctxTypePlan); uid != "" {
				issues = append(issues, ctxLintIssue{Path: doc.path, Priority: ctxLintSuggestion, Message: "task has no `plan_id` for its sourced plan (run `sdt context relations backfill`)"})
			}
		case ctxTypeAnalysis:
			issues = append(issues, checkParentListRelation(docs, doc, doc.plansIDs, ctxTypePlan, "analysis_id", "plans_ids")...)
		}
	}
	return issues
}

// checkChildRelation validates a child's typed parent id against the resolved
// parent: resolvable, right kind, and mirrored in the parent's reverse list.
func checkChildRelation(docs map[string]ctxRelDoc, doc ctxRelDoc, id, parentKind, childField, parentField string) []ctxLintIssue {
	parent, ok := docs[id]
	if !ok {
		return []ctxLintIssue{{Path: doc.path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s %s does not resolve to any document uid", childField, id)}}
	}
	if parent.kind != parentKind {
		return []ctxLintIssue{{Path: doc.path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s %s points to a %s document, want %s", childField, id, parent.kind, parentKind)}}
	}
	if !relListContains(parent, parentField, doc.uid) {
		return []ctxLintIssue{{Path: doc.path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s %s set but parent %s lacks %s %s (one-sided relation)", childField, id, parent.path, parentField, doc.uid)}}
	}
	return nil
}

// checkParentListRelation validates a parent's reverse child list: each entry
// must resolve to a child of the right kind that names this parent.
func checkParentListRelation(docs map[string]ctxRelDoc, doc ctxRelDoc, ids []string, childKind, childField, parentField string) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, id := range ids {
		child, ok := docs[id]
		if !ok {
			issues = append(issues, ctxLintIssue{Path: doc.path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s %s does not resolve to any document uid", parentField, id)})
			continue
		}
		if child.kind != childKind {
			issues = append(issues, ctxLintIssue{Path: doc.path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s %s points to a %s document, want %s", parentField, id, child.kind, childKind)})
			continue
		}
		if ctxRelField(child, childField) != doc.uid {
			issues = append(issues, ctxLintIssue{Path: doc.path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s lists %s but %s %s=%q (one-sided relation)", parentField, id, child.path, childField, ctxRelField(child, childField))})
		}
	}
	return issues
}

func ctxRelField(doc ctxRelDoc, field string) string {
	switch field {
	case "analysis_id":
		return doc.analysisID
	case "plan_id":
		return doc.planID
	}
	return ""
}

// ── context relations backfill ─────────────────────────────────────────────────

type ctxRelationsBackfillResult struct {
	Action  string   `json:"action" yaml:"action"`
	Scanned int      `json:"scanned" yaml:"scanned"`
	Linked  int      `json:"linked" yaml:"linked"`
	Skipped int      `json:"skipped" yaml:"skipped"`
	Paths   []string `json:"paths,omitempty" yaml:"paths,omitempty"`
}

// runRelationsBackfill derives the typed parent edges from `sources` and stamps
// both directions idempotently. It requires the uid backfill (the edges are
// keyed by uid) and never rewrites a uid or an existing parent id. With dryRun
// it reports the would-change files without writing.
func runRelationsBackfill(dryRun bool) (ctxRelationsBackfillResult, error) {
	res := ctxRelationsBackfillResult{Action: statusWritten}
	if dryRun {
		res.Action = statusDryRun
	}
	if !uidBackfillDone() {
		return res, fmt.Errorf("uid backfill required first: run `sdt context uid backfill` before `sdt context relations backfill`")
	}
	files, err := ctxUIDFiles()
	if err != nil {
		return res, err
	}
	uidToPath := map[string]string{}
	for _, path := range files {
		if uid := ctxDocUID(path); uid != "" {
			uidToPath[uid] = path
		}
	}

	for _, path := range files {
		act, relevant, derr := deriveRelationBackfill(path, uidToPath)
		if derr != nil {
			return res, derr
		}
		if !relevant {
			continue
		}
		res.Scanned++
		if !act.childChanged && !act.parentChanged {
			res.Skipped++
			continue
		}
		res.Linked++
		res.Paths = append(res.Paths, path)
		if dryRun {
			continue
		}
		if act.childChanged {
			if werr := writeWorkFile(act.path, act.childContent); werr != nil {
				return res, werr
			}
		}
		if act.parentChanged {
			if werr := writeWorkFile(act.parentPath, act.parentContent); werr != nil {
				return res, werr
			}
		}
	}
	return res, nil
}

// relBackfillAction is the computed (not yet applied) mutation for one child.
type relBackfillAction struct {
	path          string
	childContent  string
	childChanged  bool
	parentPath    string
	parentContent string
	parentChanged bool
}

// deriveRelationBackfill computes the typed parent edge for one document from
// its `sources` chain without writing anything. relevant is false for kinds
// outside the lifecycle chain. A missing or stale typed edge is re-derived from
// `sources`; the parent's reverse list is always reconciled.
func deriveRelationBackfill(path string, uidToPath map[string]string) (act relBackfillAction, relevant bool, err error) {
	data, err := os.ReadFile(path) //#nosec G304,G703 -- fixed repo path
	if err != nil {
		return act, false, err
	}
	content := string(data)
	kind := parseFrontmatterField(content, "kind")
	childField := ctxParentField(kind)
	if childField == "" {
		return act, false, nil
	}
	act = relBackfillAction{path: path, childContent: content}
	childUID := parseFrontmatterField(content, ctxFrontmatterUID)
	if childUID == "" {
		return act, true, nil
	}
	parentKind := ctxParentKind(kind)
	parentUID := parseFrontmatterField(content, childField)
	parentPath := ""
	if parentUID != "" {
		parentPath = uidToPath[parentUID]
	}
	if parentUID == "" || parentPath == "" {
		srcPath, srcUID := ctxSourceParent(content, parentKind)
		if srcUID == "" {
			return act, true, nil
		}
		parentPath, parentUID = srcPath, srcUID
		if parseFrontmatterField(content, childField) != parentUID {
			act.childContent, act.childChanged = stampChildParent(content, kind, parentUID)
		}
	}
	parentData, perr := os.ReadFile(parentPath) //#nosec G304,G703 -- context work file
	if perr != nil {
		return act, true, nil
	}
	act.parentPath = parentPath
	act.parentContent, act.parentChanged = appendFrontmatterListValue(string(parentData), ctxChildrenField(parentKind), childUID)
	return act, true, nil
}

// writeWorkFile persists a context work file with a checked error.
func writeWorkFile(path, content string) error {
	//#nosec G306,G703 -- user work file
	return os.WriteFile(path, []byte(content), 0o644)
}

func outputRelationsBackfill(cmd *cobra.Command, res ctxRelationsBackfillResult) {
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
		verb := "linked"
		if res.Action == statusDryRun {
			verb = "would link"
		}
		outputString(cmd, fmt.Sprintf("%s %d relation(s); %d scanned, %d already in step\n", verb, res.Linked, res.Scanned, res.Skipped))
		for _, p := range res.Paths {
			outputString(cmd, p+"\n")
		}
	}
}

var contextRelationsCmd = &cobra.Command{
	Use:   "relations",
	Short: "Manage typed parent relations",
	Long: `Manage the typed, bidirectional parent relations keyed by the document ` + "`uid`" + `.

  sdt context relations backfill [--dry-run]   derive the plan→analysis and
                                               task→plan edges from ` + "`sources`" + ` and stamp both directions`,
}

var contextRelationsBackfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Derive the typed parent relations from sources",
	Long: `Walk the ` + "`sources`" + ` derivation chain of every plan and task file and stamp
the typed parent relation in both directions: ` + "`analysis_id`" + `/` + "`plans_ids`" + ` and
` + "`plan_id`" + `/` + "`tasks_ids`" + `. Requires the uid backfill first (the edges are keyed
by uid). Idempotent; --dry-run only reports.

Examples:
  sdt context relations backfill
  sdt context relations backfill --dry-run --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		res, err := runRelationsBackfill(getBoolFlag(cmd, "dry-run", false))
		exitWithError(cmd, err)
		outputRelationsBackfill(cmd, res)
	},
}

func init() {
	contextRelationsBackfillCmd.Flags().Bool("dry-run", false, "Report without writing anything")
	contextRelationsCmd.AddCommand(contextRelationsBackfillCmd)
}

func relListContains(doc ctxRelDoc, field, value string) bool {
	var list []string
	switch field {
	case "plans_ids":
		list = doc.plansIDs
	case "tasks_ids":
		list = doc.tasksIDs
	}
	for _, v := range list {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}
