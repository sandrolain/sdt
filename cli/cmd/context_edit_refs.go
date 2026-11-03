package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── pre-write reference validation (sdt context edit --dry-run) ───────────────
//
// The dry run resolves the references the page would carry before the body is
// written: citations through the lint's resolver, body wikilinks and the
// frontmatter relations through the same builder `wiki lint` uses. It is the
// check the wave lacked — three wrong-artifact citations and one stale sha
// reached the lint only after the body was already on disk.

// contextReferenceIssue is one broken reference found before a write.
type contextReferenceIssue struct {
	Path    string
	Message string
}

// reportDryRunReferences prints the findings of a dry run and reports whether
// any were found. The caller owns the exit code.
func reportDryRunReferences(cmd *cobra.Command, path, fm, body string) bool {
	issues := validateContextReferences(path, fm, body)
	for _, issue := range issues {
		outputString(cmd, fmt.Sprintf("[WARNING] %s: %s\n", issue.Path, issue.Message))
	}
	return len(issues) > 0
}

// validateContextReferences checks the page as it would be written: the new
// body's citations and wikilinks, plus the existing frontmatter's relations
// (an edit never changes them, but a dry run reports the page it would leave).
func validateContextReferences(path, fm, body string) []contextReferenceIssue {
	var out []contextReferenceIssue
	for _, issue := range lintCitationText(path, body) {
		out = append(out, contextReferenceIssue{Path: path, Message: issue.Message})
	}
	out = append(out, claimCitationIssues(path, body)...)
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		return out
	}
	out = append(out, bodyLinkIssues(path, body, bld)...)
	out = append(out, relationIssues(path, fm, bld)...)
	return out
}

// claimCitationIssues reports a claim anchor whose line carries no citation.
func claimCitationIssues(path, body string) []contextReferenceIssue {
	var out []contextReferenceIssue
	for _, loc := range contextwiki.ClaimRegexp.FindAllStringIndex(body, -1) {
		if contextwiki.CitationRegexp.MatchString(lineAt(body, loc[0])) {
			continue
		}
		out = append(out, contextReferenceIssue{
			Path:    path,
			Message: fmt.Sprintf("claim %s has no refs/ citation", body[loc[0]:loc[1]]),
		})
	}
	return out
}

// bodyLinkIssues applies the body-link rules of `wiki lint` to the new body.
// The body is parsed alone so the frontmatter relations are not read as plain
// links twice.
func bodyLinkIssues(path, body string, bld *contextwiki.Builder) []contextReferenceIssue {
	page := contextwiki.ParseContent("---\nkind: wiki\n---\n"+body, path, "")
	var out []contextReferenceIssue
	for _, l := range page.Links {
		if l.Verb != "" && !contextwiki.Verbs[l.Verb] {
			out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("body link verb %q is outside the closed vocabulary", l.Verb)})
			continue
		}
		if l.Verb != "" && l.Target == "" {
			out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("typed link %q has no target title", l.Raw)})
			continue
		}
		if l.Verb == "" && strings.HasPrefix(l.Target, "http") {
			continue // external URL
		}
		switch {
		case len(bld.ByTitle[l.Target]) > 1:
			out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("link target %q is ambiguous (%d pages share the title)", l.Target, len(bld.ByTitle[l.Target]))})
		case len(bld.ByTitle[l.Target]) == 1, bld.ByID[l.Target] != nil:
		default:
			out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("broken link %q (no page with id or title %q)", l.Raw, l.Target)})
		}
	}
	return out
}

// relationIssues applies the relation rules of `wiki lint` to the frontmatter.
func relationIssues(path, fm string, bld *contextwiki.Builder) []contextReferenceIssue {
	page := contextwiki.ParseContent(fm+"\n", path, "")
	var out []contextReferenceIssue
	for verb, targets := range page.Relations {
		if verb == "" || !contextwiki.Verbs[verb] {
			out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("relation verb %q is outside the closed vocabulary", verb)})
			continue
		}
		for _, t := range targets {
			m := contextwiki.LinkRegexp.FindStringSubmatch(t)
			if m == nil {
				out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("relation %s target %q is not a [[id|label]] link", verb, t)})
				continue
			}
			if bld.Resolve(strings.TrimSpace(m[1])) == nil {
				out = append(out, contextReferenceIssue{Path: path, Message: fmt.Sprintf("relation %s targets missing page %q", verb, strings.TrimSpace(m[1]))})
			}
		}
	}
	return out
}
