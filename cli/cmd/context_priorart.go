package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Prior-art prefill for new analyses: when --prior-art is set, `context new`
// runs the shared search on the objective/title/topics and proposes related
// documents. It is propose-only: candidates are written into a `## Prior art`
// body section and (with --prior-art-links) into the `links` frontmatter; it
// never edits an existing document.

// priorArtHit is one proposed related document.
type priorArtHit struct {
	Path    string
	Kind    string
	Status  string
	Title   string
	Summary string
	Score   float64
}

// priorArtQuery builds the query from the objective, title and topics.
func priorArtQuery(objective, title string, topics []string) string {
	parts := []string{}
	if title != "" {
		parts = append(parts, title)
	}
	if objective != "" {
		parts = append(parts, objective)
	}
	parts = append(parts, topics...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

// collectPriorArt runs the search and returns the best candidates, excluding
// the document being created and dead-end notes.
func collectPriorArt(cmd *cobra.Command, query, selfPath string, limit int) ([]priorArtHit, error) {
	ix := ctxBuildSearchIndex(cmd)
	if query == "" {
		return nil, nil
	}
	res, err := ix.Search(query, "", "", "", "", "", "", limit)
	if err != nil {
		return nil, err
	}
	self := filepathSlash(selfPath)
	out := make([]priorArtHit, 0, len(res.Results))
	for _, r := range res.Results {
		if r.Path == self || strings.HasSuffix(r.Path, "/index.md") {
			continue
		}
		out = append(out, priorArtHit{
			Path: r.Path, Kind: r.Kind, Status: r.Status,
			Title: r.Title, Summary: r.Summary, Score: r.Score,
		})
	}
	return out, nil
}

// priorArtSection renders the `## Prior art` body block, or "" when there are
// no candidates.
func priorArtSection(hits []priorArtHit) string {
	if len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Prior art\n\n")
	b.WriteString("_Auto-proposed by `sdt context new --prior-art`; review each candidate, keep what is relevant and link it (`links`/`supersedes`/`contradicts`), then delete this note._\n\n")
	for _, h := range hits {
		meta := h.Kind
		if h.Status != "" {
			meta += "/" + h.Status
		}
		fmt.Fprintf(&b, "- [[%s]] (%s, score %.2f) — %s\n", h.Path, meta, h.Score, firstSentence(h.Summary))
	}
	b.WriteString("\n")
	return b.String()
}

// priorArtLinks extracts the candidate paths for the `links` frontmatter.
func priorArtLinks(hits []priorArtHit, max int) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if len(out) >= max {
			break
		}
		p := strings.TrimSuffix(h.Path, sdtMarkdownExt)
		p = strings.TrimPrefix(p, "context/")
		out = append(out, p)
	}
	return out
}

// firstSentence returns the first sentence of s, clamped, for compact output.
func firstSentence(s string) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if s == "" {
		return "(no summary)"
	}
	if i := strings.IndexAny(s, ".!?"); i > 0 && i < 160 {
		return s[:i+1]
	}
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

// filepathSlash normalizes a path to slash form for comparison with doc ids.
func filepathSlash(p string) string {
	return strings.ReplaceAll(p, string(os.PathSeparator), "/")
}

// injectLinks adds a `links:` frontmatter list to generated content that has
// none, preserving the rest of the frontmatter. It is only used on a
// just-generated file (never on an existing document).
func injectLinks(content string, links []string) string {
	return injectReferenceList(content, ctxFrontmatterLinks, links)
}
