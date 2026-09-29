// Package ctxrel resolves the typed lifecycle relations of the context corpus:
// a plan's single parent analysis (`analysis_id`) and a task file's single
// parent plan (`plan_id`). It is the one place that reads those fields, so the
// cascade, the lint rules, the generated index, the search manifest and the
// search registry cannot disagree about who derives from whom.
//
// The relation is singular by model decision: a plan derives from exactly one
// analysis. A `sources` entry naming another analysis is correlation, not
// derivation, and is deliberately ignored here.
package ctxrel

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// CorpusDir is the corpus root every reference is relative to.
const CorpusDir = "context"

// The lifecycle kinds, parent to child, and the directory each lives in.
const (
	kindAnalysis = "analysis"
	kindPlan     = "plan"
	kindTasks    = "tasks"

	dirAnalysis = "analysis"
	dirPlan     = "plan"
	dirTasks    = "tasks"
)

// The typed frontmatter fields the relation is keyed by.
const (
	uidField        = "uid"
	kindField       = "kind"
	analysisIDField = "analysis_id"
	planIDField     = "plan_id"
)

var lifecycleDirs = []string{dirAnalysis, dirPlan, dirTasks}

// Edges is a resolved view of the corpus lifecycle relations, keyed by
// corpus-relative reference (`context/plan/x.md`).
type Edges struct {
	parentOf   map[string]string
	childrenOf map[string][]string
}

// Load reads the lifecycle documents under the corpus dir and resolves their
// typed parent relations. A document whose parent uid is absent, empty or
// unresolvable has no parent: the corpus is not repaired here, and every
// consumer treats a childless document as underivable.
func Load(corpusDir string) (*Edges, error) {
	uidToRef := map[string]string{}
	// parent uid per child reference, filled as the directories are walked and
	// resolved against uidToRef once every document is known.
	parents := map[string]string{}

	for _, dir := range lifecycleDirs {
		if err := filepath.WalkDir(filepath.Join(corpusDir, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				// A project with no task files (or no plans yet) has no such
				// directory: the relation is simply empty, not broken.
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != contextwiki.MarkdownExt {
				return nil
			}
			ref, uid, parentUID, ok := readLifecycleDoc(path, corpusDir)
			if !ok {
				return nil
			}
			if uid != "" {
				uidToRef[uid] = ref
			}
			if parentUID != "" {
				parents[ref] = parentUID
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	e := &Edges{parentOf: map[string]string{}, childrenOf: map[string][]string{}}
	for ref, parentUID := range parents {
		parent, ok := uidToRef[parentUID]
		if parentUID == "" || !ok || parent == ref {
			continue
		}
		e.parentOf[ref] = parent
		e.childrenOf[parent] = append(e.childrenOf[parent], ref)
	}
	for parent := range e.childrenOf {
		sort.Strings(e.childrenOf[parent])
	}
	return e, nil
}

// readLifecycleDoc reads one document outside the walk callback (the same shape
// the cascade uses) and reports its corpus reference, its uid and its typed
// parent uid. parentUID is "" for a document that declares none, and ok is
// false for a document outside the lifecycle chain.
func readLifecycleDoc(path, corpusDir string) (ref, uid, parentUID string, ok bool) {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return "", "", "", false
	}
	content := string(data)
	switch contextwiki.FrontmatterField(content, kindField) {
	case kindPlan:
		return normalizeRef(path, corpusDir), contextwiki.FrontmatterField(content, uidField),
			contextwiki.FrontmatterField(content, analysisIDField), true
	case kindTasks:
		return normalizeRef(path, corpusDir), contextwiki.FrontmatterField(content, uidField),
			contextwiki.FrontmatterField(content, planIDField), true
	case kindAnalysis:
		// A parent in the chain: its uid is what a plan's `analysis_id` points at.
		return normalizeRef(path, corpusDir), contextwiki.FrontmatterField(content, uidField), "", true
	default:
		return "", "", "", false
	}
}

// ParentOf returns the single parent reference of ref, or "" when it has none
// (a plan without `analysis_id`, a task file without `plan_id`, an unresolvable
// uid, or a document outside the lifecycle chain).
func (e *Edges) ParentOf(ref string) string {
	if e == nil {
		return ""
	}
	return e.parentOf[ref]
}

// ChildrenOf returns the references whose typed parent is ref, sorted by path.
// The reverse typed lists (`plans_ids`, `tasks_ids`) are the denormalised copy
// of this relation, kept in step by `sdt context lint`; they are not read here.
func (e *Edges) ChildrenOf(ref string) []string {
	if e == nil {
		return nil
	}
	return e.childrenOf[ref]
}

// normalizeRef turns a path under the corpus dir into a corpus-relative
// reference, the form every consumer already keys its maps by.
func normalizeRef(path, corpusDir string) string {
	rel, err := filepath.Rel(corpusDir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	return CorpusDir + "/" + filepath.ToSlash(rel)
}
