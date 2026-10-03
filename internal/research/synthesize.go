package research

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// findCitationRe matches the hash-anchored citation shape used by the corpus
// (`refs/<file>@<sha>:<lines>`); synthesize emits its own simple form
// (`[<slug>@<sha8>]`) and this keeps the format recognizable to a reader.
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// Excerpt is one cited passage of a draft, attributed to a source.
type Excerpt struct {
	CanonicalURL string `json:"canonical_url" yaml:"canonical_url"`
	Title        string `json:"title,omitempty" yaml:"title,omitempty"`
	SHA8         string `json:"sha8" yaml:"sha8"`
	Citation     string `json:"citation" yaml:"citation"`
	Unverified   bool   `json:"unverified,omitempty" yaml:"unverified,omitempty"`
	Body         string `json:"body,omitempty" yaml:"body,omitempty"`
	Truncated    bool   `json:"truncated,omitempty" yaml:"truncated,omitempty"`
}

// Draft is the deterministic synthesis of a run: an outline of the sources and
// a cited excerpt per source.
type Draft struct {
	RunID     string    `json:"run_id" yaml:"run_id"`
	Query     string    `json:"query" yaml:"query"`
	Scope     string    `json:"scope,omitempty" yaml:"scope,omitempty"`
	Excerpts  []Excerpt `json:"excerpts" yaml:"excerpts"`
	Skipped   []string  `json:"skipped,omitempty" yaml:"skipped,omitempty"`
	IncludeUn bool      `json:"include_unverified,omitempty" yaml:"include_unverified,omitempty"`
}

// Synthesize assembles a cited draft from the run's sources. It only includes
// verified sources; with includeUnverified it also includes fetched/parsed
// sources, each marked as unverified. Sources without a payload, and rejected
// ones, are listed under Skipped. It is deterministic — no model, no network.
//
// runDir is the run directory; maxLines caps the excerpt per source (0 = all).
func (r *Run) Synthesize(runDir string, includeUnverified bool, maxLines int) Draft {
	d := Draft{RunID: r.RunID, Query: r.Query, Scope: r.Scope, Excerpts: []Excerpt{}, IncludeUn: includeUnverified}

	for _, s := range r.Sources {
		include := s.Status == StatusVerified
		if !include && includeUnverified && (s.Status == StatusFetched || s.Status == StatusParsed) {
			include = true
		}
		if !include {
			if s.Status != StatusDiscovered {
				d.Skipped = append(d.Skipped, fmt.Sprintf("%s (%s)", s.CanonicalURL, s.Status))
			}
			continue
		}
		if s.RawPath == "" {
			d.Skipped = append(d.Skipped, fmt.Sprintf("%s (no payload)", s.CanonicalURL))
			continue
		}
		abs := filepath.Join(runDir, filepath.FromSlash(s.RawPath))
		data, err := os.ReadFile(abs) //#nosec G304 -- raw path recorded in the manifest
		if err != nil {
			d.Skipped = append(d.Skipped, fmt.Sprintf("%s (payload unreadable)", s.CanonicalURL))
			continue
		}
		body, truncated := excerptBody(string(data), maxLines)
		d.Excerpts = append(d.Excerpts, Excerpt{
			CanonicalURL: s.CanonicalURL,
			Title:        s.Title,
			SHA8:         shortSHA(s.SHA256),
			Citation:     fmt.Sprintf("[%s@%s]", SourceSlug(s.CanonicalURL), shortSHA(s.SHA256)),
			Unverified:   s.Status != StatusVerified,
			Body:         body,
			Truncated:    truncated,
		})
	}
	sort.Slice(d.Excerpts, func(i, j int) bool { return d.Excerpts[i].CanonicalURL < d.Excerpts[j].CanonicalURL })
	return d
}

// excerptBody strips the crawldown frontmatter and returns up to maxLines lines
// of the payload plus whether it was truncated.
func excerptBody(markdown string, maxLines int) (string, bool) {
	body := markdown
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---\n"); end >= 0 {
			body = body[4+end+5:]
		}
	}
	body = strings.TrimSpace(body)
	if maxLines <= 0 {
		return body, false
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= maxLines {
		return body, false
	}
	return strings.Join(lines[:maxLines], "\n"), true
}

// Render renders the draft as Markdown with a heading, the outline and one
// cited excerpt per included source. Unverified excerpts are flagged.
func (d Draft) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Research draft: %s\n\n", d.Query)
	if d.Scope != "" {
		fmt.Fprintf(&b, "_Scope: %s_\n\n", d.Scope)
	}
	fmt.Fprintf(&b, "Run `%s`. %d cited source(s).\n\n", d.RunID, len(d.Excerpts))

	b.WriteString("## Sources\n\n")
	if len(d.Excerpts) == 0 {
		b.WriteString("_No verified sources: run `sdt research verify` after fetching._\n\n")
	}
	for _, e := range d.Excerpts {
		flag := ""
		if e.Unverified {
			flag = " **UNVERIFIED**"
		}
		title := e.Title
		if title == "" {
			title = e.CanonicalURL
		}
		fmt.Fprintf(&b, "- %s %s%s\n", title, e.Citation, flag)
	}

	b.WriteString("\n## Excerpts\n\n")
	for _, e := range d.Excerpts {
		flag := ""
		if e.Unverified {
			flag = " (unverified)"
		}
		fmt.Fprintf(&b, "### %s%s\n\n", e.CanonicalURL, flag)
		fmt.Fprintf(&b, "Source: %s — %s\n\n", e.CanonicalURL, e.Citation)
		if e.Body != "" {
			b.WriteString(e.Body)
			if e.Truncated {
				b.WriteString("\n\n_…excerpt truncated_")
			}
			b.WriteString("\n\n")
		}
	}

	if len(d.Skipped) > 0 {
		b.WriteString("## Skipped\n\n")
		for _, s := range d.Skipped {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	return b.String()
}
