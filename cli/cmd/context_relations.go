package cmd

import (
	"fmt"
	"os"
	"strings"
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
	return parseFrontmatterField(string(data), "uid")
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
	data, err := os.ReadFile(parentPath) //#nosec G304 -- context work file
	if err != nil {
		return err
	}
	updated, changed := appendFrontmatterListValue(string(data), field, childUID)
	if !changed {
		return nil
	}
	//#nosec G306 -- user work file
	return os.WriteFile(parentPath, []byte(updated), 0o644)
}

// stampChildParent injects the typed parent scalar on a child document content
// from the parent uid. Returns the content unchanged when either id is missing.
func stampChildParent(content, childKind, parentUID string) (string, bool) {
	field := ctxParentField(childKind)
	childUID := parseFrontmatterField(content, "uid")
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
			uid:        parseFrontmatterField(content, "uid"),
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
	warn := func(path, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: msg})
	}
	suggest := func(path, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: msg})
	}

	// Children declare their parent id and must agree with the parent's list.
	checkChild := func(doc ctxRelDoc, id, parentKind, childField, parentField string) {
		parent, ok := docs[id]
		if !ok {
			warn(doc.path, fmt.Sprintf("%s %s does not resolve to any document uid", childField, id))
			return
		}
		if parent.kind != parentKind {
			warn(doc.path, fmt.Sprintf("%s %s points to a %s document, want %s", childField, id, parent.kind, parentKind))
			return
		}
		if !relListContains(parent, parentField, doc.uid) {
			warn(doc.path, fmt.Sprintf("%s %s set but parent %s lacks %s %s (one-sided relation)", childField, id, parent.path, parentField, doc.uid))
		}
	}
	// Parents' reverse lists must agree with the child declaration.
	checkParentList := func(doc ctxRelDoc, ids []string, childKind, childField, parentField string) {
		for _, id := range ids {
			child, ok := docs[id]
			if !ok {
				warn(doc.path, fmt.Sprintf("%s %s does not resolve to any document uid", parentField, id))
				continue
			}
			if child.kind != childKind {
				warn(doc.path, fmt.Sprintf("%s %s points to a %s document, want %s", parentField, id, child.kind, childKind))
				continue
			}
			if ctxRelField(child, childField) != doc.uid {
				warn(doc.path, fmt.Sprintf("%s lists %s but %s %s=%q (one-sided relation)", parentField, id, child.path, childField, ctxRelField(child, childField)))
			}
		}
	}

	for _, doc := range all {
		switch doc.kind {
		case ctxTypePlan:
			if doc.analysisID != "" {
				checkChild(doc, doc.analysisID, ctxTypeAnalysis, "analysis_id", "plans_ids")
			} else if _, uid := ctxSourceParent(doc.content, ctxTypeAnalysis); uid != "" {
				suggest(doc.path, "plan has no `analysis_id` for its sourced analysis (run `sdt context relations backfill`)")
			}
			checkParentList(doc, doc.tasksIDs, ctxTypeTasks, "plan_id", "tasks_ids")
		case ctxTypeTasks:
			if doc.planID != "" {
				checkChild(doc, doc.planID, ctxTypePlan, "plan_id", "tasks_ids")
			} else if _, uid := ctxSourceParent(doc.content, ctxTypePlan); uid != "" {
				suggest(doc.path, "task has no `plan_id` for its sourced plan (run `sdt context relations backfill`)")
			}
		case ctxTypeAnalysis:
			checkParentList(doc, doc.plansIDs, ctxTypePlan, "analysis_id", "plans_ids")
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

