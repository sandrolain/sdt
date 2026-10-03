package research

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WikiPage is one proposed wiki page derived from a verified research source.
type WikiPage struct {
	ID       string   `json:"id" yaml:"id"`
	Title    string   `json:"title" yaml:"title"`
	Action   string   `json:"action" yaml:"action"` // create | update | conflict
	Reason   string   `json:"reason,omitempty" yaml:"reason,omitempty"`
	Evidence []string `json:"evidence" yaml:"evidence"` // citations of the sources backing the page
}

// WikiPlan is the proposal of populate-wiki: what would be created or changed,
// and what was refused. It is produced with no writes.
type WikiPlan struct {
	RunID    string     `json:"run_id" yaml:"run_id"`
	Brief    string     `json:"brief,omitempty" yaml:"brief,omitempty"`
	Pages    []WikiPage `json:"pages" yaml:"pages"`
	Refused  []string   `json:"refused,omitempty" yaml:"refused"`
	MaxPages int        `json:"max_pages,omitempty" yaml:"max_pages,omitempty"`
}

// Establishable reports whether the plan proposes at least one page.
func (p WikiPlan) Establishable() bool { return len(p.Pages) > 0 }

// PlanWiki builds the promotion plan for a run: one page per verified source
// (never from an unverified claim), deduplicated against the existing wiki ids.
// It reads existingIDs (the ids already present under wiki/) to classify an
// action as create or update; a source whose slug already exists is an update.
// A page budget (maxPages > 0) caps how many pages are proposed; the rest are
// refused with the reason.
//
// It performs no writes; applying the plan is the caller's, gated, step.
func (r *Run) PlanWiki(runDir string, brief string, existingIDs []string, maxPages int) WikiPlan {
	plan := WikiPlan{RunID: r.RunID, Brief: brief, Pages: []WikiPage{}, MaxPages: maxPages}
	existing := map[string]bool{}
	for _, id := range existingIDs {
		existing[id] = true
	}

	for _, s := range r.Sources {
		if s.Status != StatusVerified {
			// Never propose a page from a source SDT could not verify.
			plan.Refused = append(plan.Refused, fmt.Sprintf("%s (%s: not verified)", s.CanonicalURL, s.Status))
			continue
		}
		if len(plan.Pages) >= maxPages && maxPages > 0 {
			plan.Refused = append(plan.Refused, fmt.Sprintf("%s (page budget %d reached)", s.CanonicalURL, maxPages))
			continue
		}
		id := SourceSlug(s.CanonicalURL)
		page := WikiPage{
			ID:       id,
			Title:    wikiTitle(s),
			Evidence: []string{fmt.Sprintf("[%s@%s]", id, shortSHA(s.SHA256))},
			Reason:   "verified research source",
		}
		if existing[id] {
			page.Action = "update"
		} else {
			page.Action = "create"
		}
		plan.Pages = append(plan.Pages, page)
	}
	sort.Slice(plan.Pages, func(i, j int) bool { return plan.Pages[i].ID < plan.Pages[j].ID })
	return plan
}

func wikiTitle(s Source) string {
	if strings.TrimSpace(s.Title) != "" {
		return strings.TrimSpace(s.Title)
	}
	return s.CanonicalURL
}

// RenderPlan renders the human-readable preview of the plan (no writes): the
// proposed pages with their action and evidence, and the refused sources.
func (p WikiPlan) RenderPlan() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wiki promotion plan for run %s\n", p.RunID)
	if p.Brief != "" {
		fmt.Fprintf(&b, "  brief: %s\n", p.Brief)
	}
	fmt.Fprintf(&b, "  pages: %d", len(p.Pages))
	if p.MaxPages > 0 {
		fmt.Fprintf(&b, " (budget %d)", p.MaxPages)
	}
	b.WriteString("\n")
	for _, page := range p.Pages {
		fmt.Fprintf(&b, "  - [%s] %s — %s\n", page.Action, page.ID, page.Title)
		for _, e := range page.Evidence {
			fmt.Fprintf(&b, "      evidence: %s\n", e)
		}
	}
	if len(p.Refused) > 0 {
		b.WriteString("  refused:\n")
		for _, r := range p.Refused {
			fmt.Fprintf(&b, "    - %s\n", r)
		}
	}
	return b.String()
}

// RenderPage renders the Markdown of a proposed wiki page for the given source,
// carrying its evidence in `sources`. refRel is the source's archived path under
// context/refs/ (e.g. `refs/<slug>.md`), which the claim cites so the wiki
// contract's `refs/` citation requirement is satisfied. It is used only by the
// applying step.
func RenderPage(page WikiPage, s Source, runID, refRel string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nkind: wiki\nid: %s\ntitle: %q\ntype: concept\nstatus: draft\nsummary: %q\ntags:\n  - research\n  - %s\nsources:\n  - %s\n  - research run %s\n---\n\n",
		page.ID, page.Title, "Promoted from research run "+runID, tagSlug(page.ID), refRel, runID)
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", page.Title)
	fmt.Fprintf(&b, "## Claims\n\n1. %s (%s@%s) {#claim-1}.\n\n",
		page.Title, refRel, shortSHA(s.SHA256))
	return b.String()
}

// tagSlug derives a wiki tag from a page id (the corpus uses slash-namespaced
// slugs; a bare id is prefixed with the research origin).
func tagSlug(id string) string {
	return "research/" + strings.TrimSpace(id)
}

// WikiPagePath returns the on-disk path of a wiki page id under a project root.
func WikiPagePath(root, id string) string {
	return filepath.Join(root, "context", "wiki", filepath.FromSlash(id)+".md")
}

// RefRelPath returns the archive path of a source under context/refs/, relative
// to context/ (the shape wiki citations use: `refs/<slug>.md`).
func RefRelPath(s Source) string {
	return "refs/" + SourceSlug(s.CanonicalURL) + ".md"
}

// ArchiveSource copies a source's raw payload from the run into context/refs/ so
// the archived file is the immutable capture a wiki claim cites. It is
// idempotent: an existing refs file is left untouched. It returns the refs path
// relative to context/ (e.g. `refs/<slug>.md`).
func (r *Run) ArchiveSource(root string, s Source) (string, error) {
	if s.RawPath == "" {
		return "", fmt.Errorf("source %s has no raw payload to archive", s.CanonicalURL)
	}
	rel := RefRelPath(s)
	abs := filepath.Join(root, "context", filepath.FromSlash(rel))
	if _, err := os.Stat(abs); err == nil {
		return rel, nil // already archived
	}
	raw, err := os.ReadFile(filepath.Join(RunDir(root, r.RunID), filepath.FromSlash(s.RawPath))) //#nosec G304 -- run raw path
	if err != nil {
		return "", fmt.Errorf("read raw payload: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, raw, 0o644); err != nil { //#nosec G306,G703 -- immutable corpus capture under context/refs/
		return "", err
	}
	return rel, nil
}
