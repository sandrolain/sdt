package research

import (
	"fmt"
	"strings"
)

// ArchiveEntry is one verified capture planned for (or written to) a run's
// dated, objective-scoped directory under context/refs/.
type ArchiveEntry struct {
	CanonicalURL string `json:"canonical_url" yaml:"canonical_url"`
	Title        string `json:"title,omitempty" yaml:"title,omitempty"`
	Ref          string `json:"ref" yaml:"ref"`           // refs/<dir>/<result>.md
	Citation     string `json:"citation" yaml:"citation"` // refs/<dir>/<result>.md@<sha8>
}

// ArchivePlan is the proposal of `sdt research archive`: the verified captures
// to promote into the run's dated refs directory, and what was refused. It is
// produced with no writes.
type ArchivePlan struct {
	RunID      string         `json:"run_id" yaml:"run_id"`
	ArchiveDir string         `json:"archive_dir" yaml:"archive_dir"`
	Entries    []ArchiveEntry `json:"entries" yaml:"entries"`
	Refused    []string       `json:"refused,omitempty" yaml:"refused"`
}

// Establishable reports whether the plan has at least one capture to archive.
func (p ArchivePlan) Establishable() bool { return len(p.Entries) > 0 }

// PlanArchive builds the archive plan for a run: one entry per verified source,
// citing the capture's refs path and hash. A source SDT could not verify is
// refused with the reason, so it can never enter the immutable corpus. It
// performs no writes.
func (r *Run) PlanArchive() ArchivePlan {
	plan := ArchivePlan{RunID: r.RunID, ArchiveDir: ArchiveRefDir(r), Entries: []ArchiveEntry{}}
	for _, s := range r.Sources {
		if s.Status != StatusVerified {
			plan.Refused = append(plan.Refused, fmt.Sprintf("%s (%s: not verified)", s.CanonicalURL, s.Status))
			continue
		}
		ref := RefRelPath(r, s)
		plan.Entries = append(plan.Entries, ArchiveEntry{
			CanonicalURL: s.CanonicalURL,
			Title:        wikiTitle(s),
			Ref:          ref,
			Citation:     fmt.Sprintf("%s@%s", ref, shortSHA(s.SHA256)),
		})
	}
	return plan
}

// RenderPlan renders the human-readable preview of the archive (no writes): the
// target directory, the citation pack and the refused sources.
func (p ArchivePlan) RenderPlan() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Archive plan for run %s\n", p.RunID)
	fmt.Fprintf(&b, "  archive dir: %s\n", p.ArchiveDir)
	fmt.Fprintf(&b, "  captures: %d\n", len(p.Entries))
	for _, e := range p.Entries {
		fmt.Fprintf(&b, "  - %s\n", e.Citation)
	}
	if len(p.Refused) > 0 {
		b.WriteString("  refused:\n")
		for _, r := range p.Refused {
			fmt.Fprintf(&b, "    - %s\n", r)
		}
	}
	return b.String()
}

// RenderApplied renders the human-readable result of an applied archive: the
// count, the target directory and the citation pack the agent cites.
func (p ArchivePlan) RenderApplied() string {
	var b strings.Builder
	fmt.Fprintf(&b, "archived %d capture(s) under %s\n", len(p.Entries), p.ArchiveDir)
	for _, e := range p.Entries {
		fmt.Fprintf(&b, "  - %s\n", e.Citation)
	}
	return b.String()
}
