package research

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WikiBrief is the read-only inventory `sdt research populate-wiki` prints: the
// verified sources an agent distils into wiki pages, with the citation each page
// must carry. The CLI never writes a page from it — populating the wiki is an
// agent editorial task per context/instructions/ingestion.md.
type WikiBrief struct {
	RunID      string         `json:"run_id" yaml:"run_id"`
	Brief      string         `json:"brief,omitempty" yaml:"brief,omitempty"`
	ArchiveDir string         `json:"archive_dir" yaml:"archive_dir"`
	Sources    []ArchiveEntry `json:"sources" yaml:"sources"`
	Refused    []string       `json:"refused,omitempty" yaml:"refused"`
}

// PlanWikiBrief builds the brief for a run from its verified sources. It is
// read-only; the sources carry the citation each authored page must cite.
func (r *Run) PlanWikiBrief(brief string) WikiBrief {
	plan := r.PlanArchive()
	return WikiBrief{
		RunID:      r.RunID,
		Brief:      brief,
		ArchiveDir: plan.ArchiveDir,
		Sources:    plan.Entries,
		Refused:    plan.Refused,
	}
}

// Render renders the human-readable brief: the citation pack, the refusal list
// and the editorial reminder.
func (w WikiBrief) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wiki brief for run %s\n", w.RunID)
	if w.Brief != "" {
		fmt.Fprintf(&b, "  brief: %s\n", w.Brief)
	}
	fmt.Fprintf(&b, "  archive dir: %s\n", w.ArchiveDir)
	fmt.Fprintf(&b, "  verified sources: %d\n", len(w.Sources))
	for _, e := range w.Sources {
		fmt.Fprintf(&b, "  - %s — %s\n", e.Citation, e.Title)
	}
	if len(w.Refused) > 0 {
		b.WriteString("  refused:\n")
		for _, r := range w.Refused {
			fmt.Fprintf(&b, "    - %s\n", r)
		}
	}
	b.WriteString("  author pages editorially per context/instructions/ingestion.md; the CLI never writes a page\n")
	return b.String()
}

func wikiTitle(s Source) string {
	if strings.TrimSpace(s.Title) != "" {
		return strings.TrimSpace(s.Title)
	}
	return s.CanonicalURL
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
