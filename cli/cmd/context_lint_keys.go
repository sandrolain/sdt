//nolint:goconst // the declared-key set is inherently a table of string keys
package cmd

import (
	"sort"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ctxDeclaredKeys is the set of top-level frontmatter keys the corpus contract
// declares: the registry-driven fields written by `context new`/`context task`
// plus the CLI-managed and cross-cutting keys. A key outside this set is not an
// error — `sdt context set` is free with respect to keys — but lint surfaces it
// as a SUGGESTION so the schema stays discoverable without being closed.
var ctxDeclaredKeys = map[string]bool{
	// Identity and type.
	ctxFrontmatterKind: true, ctxFrontmatterUID: true, ctxFrontmatterNumber: true, ctxDocTitleKey: true,
	"summary": true, "context": true, ctxFrontmatterObjective: true, "phase": true, "phases": true,
	ctxFrontmatterTopics: true, "entities": true, "categories": true,
	// Lifecycle.
	ctxMapStatus: true, "created": true, statusUpdated: true,
	// Relations.
	ctxFrontmatterLinks: true, "sources": true, "supersedes": true, ctxFrontmatterContradicts: true,
	ctxKeyAnalysisID: true, ctxKeyPlanID: true, ctxKeyPlansIDs: true, ctxKeyTasksIDs: true,
	"derived_from": true, "results": true,
	// Provenance and annotations.
	ctxFrontmatterAgent: true, "model": true, "session": true, "role": true, "note_type": true,
	"via": true, "project": true, "group": true, "id": true, "tier": true,
	// Index/viewer generated markers.
	"_generated": true, "position": true, "order": true,
	// Document-type-specific fields used by existing templates/docs.
	"component": true, "procedure": true, "subject": true, "image": true,
	"tasks": true, "summary_sources_note": true,
	// Kept-document provenance (refs/, out of the work-file lint path but
	// declared here so the set is complete for any caller).
	"url": true, "saved_at": true, "sha256": true, "converter": true, "converted_at": true,
	"date": true, "description": true, "keywords": true, "author": true,
}

// lintUndeclaredFrontmatterKeys raises an advisory SUGGESTION for every
// top-level frontmatter key outside ctxDeclaredKeys. Dwelling kinds (notes,
// worklog, reference) are included: the schema is discoverable everywhere.
func lintUndeclaredFrontmatterKeys(path, content string) []ctxLintIssue {
	values := contextwiki.FrontmatterValues(content)
	if len(values) == 0 {
		return nil
	}
	var unknown []string
	for key := range values {
		if !ctxDeclaredKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	issues := make([]ctxLintIssue, 0, len(unknown))
	for _, key := range unknown {
		issues = append(issues, ctxLintIssue{
			Path:     path,
			Priority: ctxLintSuggestion,
			Message:  "undeclared frontmatter key `" + key + "`",
		})
	}
	return issues
}
