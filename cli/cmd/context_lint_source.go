package cmd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Source sheets (`context/sources/<slug>.md`).
//
// A sheet is the catalogue entry for one external reference, and only three of
// its keys carry a shape worth refusing: the URL every sheet must have, the
// digest that pins captured bytes, and the licence identifier that a template
// copies verbatim. Everything else is prose or a snapshot, and a check that
// guesses at a snapshot is a check an author learns to ignore
// (`context/instructions/authoring.md`).

// ctxSourceURLKey is the one required key of a source sheet.
const ctxSourceURLKey = "url"

// ctxSourceSPDXRegexp is the SPDX identifier shape: an alphanumeric head, then
// the characters an identifier may carry. The vocabulary itself is upstream and
// open-ended (`MIT`, `Apache-2.0`, `AGPL-3.0-or-later`, `CC-BY-4.0`), so the
// check binds the form, never a closed list that would refuse a valid licence.
var ctxSourceSPDXRegexp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]*$`)

// ctxSourceSHA256Regexp is a lowercase hex SHA-256, the `.source.yaml` shape.
var ctxSourceSHA256Regexp = regexp.MustCompile(`^[0-9a-f]{64}$`)

// lintSourceSheet checks the three keys of a source sheet that have a form.
func lintSourceSheet(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeSource {
		return nil
	}
	var issues []ctxLintIssue

	raw := strings.TrimSpace(parseFrontmatterField(content, ctxSourceURLKey))
	switch raw {
	case "":
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning,
			Message: "source sheet missing `url` (a source without an address is not a source)"})
	default:
		u, err := url.Parse(raw)
		//nolint:goconst // "http" is a URL scheme here; it collides with two cobra command names, not with a constant
		if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning,
				Message: fmt.Sprintf("`url` %q is not an absolute http(s) URL", raw)})
		}
	}

	if digest := strings.TrimSpace(parseFrontmatterField(content, "sha256")); digest != "" && !ctxSourceSHA256Regexp.MatchString(digest) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning,
			Message: "`sha256` is not a lowercase hex SHA-256 (64 characters)"})
	}

	if licence := strings.TrimSpace(parseFrontmatterField(content, "license")); licence != "" && !ctxSourceSPDXRegexp.MatchString(licence) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning,
			Message: fmt.Sprintf("`license` %q is not an SPDX identifier (e.g. MIT, Apache-2.0, AGPL-3.0-or-later)", licence)})
	}

	// A restore command on a source that is still live is dead weight the next
	// reader has to classify; the field belongs to the custody transition.
	status := strings.TrimSpace(parseFrontmatterField(content, "status"))
	if strings.TrimSpace(parseFrontmatterField(content, "restore")) != "" && status != "" && status != ctxSourceStatusGone {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion,
			Message: "`restore` recorded while status is not `gone` (keep it for the transition, or move it to the custody notes)"})
	}

	return issues
}
