package research

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// ArchiveDirName is the dated, objective-scoped directory a run's captures are
// archived into: <YYYYMMDD-HHMMSS>-<objective>. The timestamp is the run's
// Created instant rendered in UTC; an unparseable Created degrades to now. The
// objective slug is never empty (ObjectiveSlug falls back to "research"), so the
// name is always a single valid path segment.
func ArchiveDirName(r *Run) string {
	stamp := ""
	if r != nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(r.Created)); err == nil {
			stamp = t.UTC().Format("20060102-150405")
		}
	}
	if stamp == "" {
		stamp = time.Now().UTC().Format("20060102-150405")
	}
	return stamp + "-" + ObjectiveSlug(r)
}

// ArchiveRefDir returns the run's archive directory relative to context/, e.g.
// `refs/20261004-113000-web-capture-tooling`.
func ArchiveRefDir(r *Run) string {
	return "refs/" + ArchiveDirName(r)
}

// ResultSlug derives the archive filename stem for a source: its title slug,
// falling back to the URL's last path segment (then host). It is never empty.
func ResultSlug(s Source) string {
	if t := slugifySlug(s.Title); t != "" {
		return t
	}
	return urlSlug(s.CanonicalURL)
}

// ArchiveFileName returns the archive filename for a source inside a run's
// archive directory: <ResultSlug>.md, with a -<sha256[:8]> suffix only when
// another source in the same run yields the same result name (so distinct
// sources stay distinct without an always-on hash).
func ArchiveFileName(r *Run, s Source) string {
	stem := ResultSlug(s)
	if r != nil {
		for i := range r.Sources {
			other := r.Sources[i]
			if other.CanonicalURL == s.CanonicalURL {
				continue
			}
			if ResultSlug(other) == stem {
				if suf := shortSHA(s.SHA256); suf != "" {
					stem += "-" + suf
				}
				break
			}
		}
	}
	return stem + ".md"
}

// RefRelPath returns the archive path of a source under context/refs/, relative
// to context/ (the shape wiki citations use: `refs/<dir>/<result>.md`).
func RefRelPath(r *Run, s Source) string {
	return ArchiveRefDir(r) + "/" + ArchiveFileName(r, s)
}

// ArchiveSource copies a source's raw payload from the run into the run's dated
// directory under context/refs/, so the archived file is the immutable capture a
// wiki claim cites. It returns the refs path relative to context/ (e.g.
// `refs/20261004-113000-web-capture-tooling/page.md`).
//
// It refuses a pre-existing archive directory that is not this run's own
// (recorded in Run.ArchiveDir) without writing, and is otherwise idempotent: an
// existing file with the same content is left untouched.
func (r *Run) ArchiveSource(root string, s Source) (string, error) {
	if s.RawPath == "" {
		return "", fmt.Errorf("source %s has no raw payload to archive", s.CanonicalURL)
	}
	raw, err := os.ReadFile(filepath.Join(RunDir(root, r.RunID), filepath.FromSlash(s.RawPath))) //#nosec G304 -- run raw path
	if err != nil {
		return "", fmt.Errorf("read raw payload: %w", err)
	}

	dir := ArchiveRefDir(r)
	rel := dir + "/" + ArchiveFileName(r, s)
	absDir := filepath.Join(root, "context", filepath.FromSlash(dir))
	abs := filepath.Join(root, "context", filepath.FromSlash(rel))

	// D4: if the target directory already exists it must be this run's own
	// recorded directory; a foreign same-named directory is refused, no write.
	if _, statErr := os.Stat(absDir); statErr == nil {
		if r.ArchiveDir == "" || r.ArchiveDir != dir {
			return "", fmt.Errorf("archive directory %s already exists and is not this run's; refusing to write", dir)
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}

	if existing, readErr := os.ReadFile(abs); readErr == nil { //#nosec G304 -- archive path under context/refs
		if string(existing) == string(raw) {
			r.ArchiveDir = dir
			return rel, nil // already archived, same content
		}
		return "", fmt.Errorf("archive file %s already exists with different content; refusing to overwrite", rel)
	} else if !os.IsNotExist(readErr) {
		return "", readErr
	}

	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, raw, 0o644); err != nil { //#nosec G306,G703 -- immutable corpus capture under context/refs/
		return "", err
	}
	r.ArchiveDir = dir
	return rel, nil
}
