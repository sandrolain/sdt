package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ── agent roles check (Wave 3 phase 5) ────────────────────────────────────────
//
// Deterministic, no-runtime validation of the generated role profiles under
// context/roles/. Three checks:
//
//   - register closure + profile presence: the on-disk set must match the closed
//     register exactly (shared.md + one <slug>.md per register slug);
//   - drift guard: each profile's generated layers (core, project) must match
//     what the renderer produces for the same project and created timestamp;
//     the user-owned preferences scope is excluded from comparison;
//   - owner/reviewer overlap: no path (register `Owned` or declared repo paths)
//     may be claimed by more than one role.
//
// The same findings are mirrored into `context lint` as advisory SUGGESTIONs
// (see lintRoleProfiles), so lint stays non-failing while `agent roles check`
// is the strict gate-friendly surface.

// roleCheckFinding is one result of `agent roles check`.
type roleCheckFinding struct {
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Priority string `json:"priority" yaml:"priority"`
	Message  string `json:"message" yaml:"message"`
	Hint     string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

// Remediation hints shared by the check, doctor mirror and lint mirror.
const (
	roleCheckHintMissing      = "run `sdt agent roles init` to generate the missing profile"
	roleCheckHintUnregistered = "remove the file or register the role in cli/cmd/agentroles.go"
	roleCheckHintDrift        = "run `sdt agent roles init --force` to regenerate the generated layers"
	roleCheckHintCreated      = "fix the `created` frontmatter (RFC3339) or regenerate with `sdt agent roles init --force`"
	roleCheckHintOverlap      = "one role owns each path and another reviews it; resolve in the project layers or the register"
)

// roleCheckFindings aggregates every deterministic role check in a stable order
// (path, then priority).
func roleCheckFindings() []roleCheckFinding {
	findings := []roleCheckFinding{}
	findings = append(findings, roleCheckRegisterPresence()...)
	findings = append(findings, roleCheckDrift()...)
	findings = append(findings, roleCheckOverlap()...)
	p := map[string]int{ctxLintCritical: 0, ctxLintWarning: 1, ctxLintSuggestion: 2}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return p[findings[i].Priority] < p[findings[j].Priority]
	})
	return findings
}

// roleCheckRegisterPresence verifies the on-disk profile set against the closed
// register: shared.md plus one <slug>.md per register slug, nothing more.
func roleCheckRegisterPresence() []roleCheckFinding {
	expected := map[string]bool{sdtRolesSharedFile: true}
	for _, r := range roleRegister {
		expected[r.Slug+sdtMarkdownExt] = true
	}
	got := map[string]string{}
	files, err := dirFiles(sdtRolesDir)
	if err != nil {
		return []roleCheckFinding{{Path: sdtRolesDir, Priority: ctxLintCritical, Message: "cannot list role profiles: " + err.Error(), Hint: "check the directory permissions"}}
	}
	for _, p := range files {
		got[filepath.Base(p)] = p
	}

	var findings []roleCheckFinding
	for base := range expected {
		if _, ok := got[base]; !ok {
			findings = append(findings, roleCheckFinding{Path: filepath.Join(sdtRolesDir, base), Priority: ctxLintCritical, Message: "missing role profile", Hint: roleCheckHintMissing})
		}
	}
	for base, path := range got {
		if !expected[base] {
			findings = append(findings, roleCheckFinding{Path: path, Priority: ctxLintCritical, Message: "unregistered role profile", Hint: roleCheckHintUnregistered})
		}
	}
	return findings
}

// roleCheckDrift compares each profile's generated layers to a fresh render for
// the same project/created timestamp. Only the core and project layers are
// compared; the user-owned preferences scope is never treated as drift.
func roleCheckDrift() []roleCheckFinding {
	var findings []roleCheckFinding
	for _, r := range roleRegister {
		path := filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt)
		data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir
		if err != nil {
			continue // presence check reports the missing file
		}
		content := string(data)
		if got, want := parseFrontmatterField(content, "slug"), r.Slug; got != want {
			findings = append(findings, roleCheckFinding{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("frontmatter slug %q does not match register slug %q", got, want), Hint: roleCheckHintDrift})
		}
		created := parseFrontmatterField(content, "created")
		ts, perr := time.Parse(time.RFC3339, created)
		if perr != nil {
			findings = append(findings, roleCheckFinding{Path: path, Priority: ctxLintWarning, Message: "unparsable `created` frontmatter (want RFC3339)", Hint: roleCheckHintCreated})
			continue
		}
		project := parseFrontmatterField(content, "project")
		want, ok := renderRoleProfile(r, project, ts)
		if !ok {
			findings = append(findings, roleCheckFinding{Path: path, Priority: ctxLintCritical, Message: "cannot render profile for role " + r.Slug, Hint: roleCheckHintDrift})
			continue
		}
		for _, layer := range []string{roleLayerCore, roleLayerProject} {
			name := roleSectionName(r.Slug, layer)
			gotBody, gotOK := sectionBody(content, name)
			wantBody, wantOK := sectionBody(want, name)
			if !gotOK {
				findings = append(findings, roleCheckFinding{Path: path, Priority: ctxLintCritical, Message: "missing " + layer + " layer marker", Hint: roleCheckHintDrift})
				continue
			}
			if wantOK && gotBody != wantBody {
				findings = append(findings, roleCheckFinding{Path: path, Priority: ctxLintCritical, Message: "role profile " + layer + " layer drifted from the template", Hint: roleCheckHintDrift})
			}
		}
	}
	return findings
}

// sectionBody returns the body between the begin/end markers of the named
// section, trimmed, or ("", false) when either marker is absent.
func sectionBody(content, name string) (string, bool) {
	begin := sectionBeginMarker(name)
	end := sectionEndMarker(name)
	bi := strings.Index(content, begin)
	if bi < 0 {
		return "", false
	}
	rest := content[bi+len(begin):]
	ei := strings.Index(rest, end)
	if ei < 0 {
		return "", false
	}
	return strings.TrimSpace(rest[:ei]), true
}

// ownedRepoPathLine matches the project-layer declaration line carrying the
// backtick-delimited list of repo paths a role owns.
var ownedRepoPathLine = regexp.MustCompile(`(?m)^- Owned repo paths:\s*(.*)$`)

// roleDeclaredOwned returns the paths a role declares as its own: the register
// `Owned` default plus any "Owned repo paths" list in the on-disk project layer.
func roleDeclaredOwned(r roleDescriptor) []string {
	out := append([]string{}, r.Owned...)
	path := filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt)
	data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir
	if err != nil {
		return out
	}
	body, ok := sectionBody(string(data), roleSectionName(r.Slug, roleLayerProject))
	if !ok {
		return out
	}
	if m := ownedRepoPathLine.FindStringSubmatch(body); m != nil {
		out = append(out, backtickList(m[1])...)
	}
	return out
}

// backtickToken extracts the backtick-delimited tokens of a string.
var backtickToken = regexp.MustCompile("`([^`]+)`")

// uniqueStrings removes duplicates from a slice, preserving first-occurrence
// order.
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// backtickList returns the backtick-delimited tokens of a string.
func backtickList(s string) []string {
	var out []string
	for _, m := range backtickToken.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// roleCheckOverlap flags every path claimed by more than one role: register
// `Owned` defaults plus the declared repo paths. Overlap violates the shared
// rules ("two roles must never own the same path; one role owns, another
// reviews"), so it fails the check.
func roleCheckOverlap() []roleCheckFinding {
	owners := map[string][]string{}
	for _, r := range roleRegister {
		for _, p := range roleDeclaredOwned(r) {
			owners[p] = append(owners[p], r.Slug)
		}
	}
	var findings []roleCheckFinding
	for p, roles := range owners {
		u := uniqueStrings(roles)
		if len(u) < 2 {
			continue
		}
		findings = append(findings, roleCheckFinding{Path: p, Priority: ctxLintCritical, Message: "owned path overlap: claimed by " + strings.Join(u, ", "), Hint: roleCheckHintOverlap})
	}
	return findings
}

// lintRoleProfiles mirrors the deterministic role checks into `context lint` as
// advisory SUGGESTIONs: lint stays non-failing while `agent roles check` is the
// strict gate surface. Profiles not generated yet are not flagged: `agent roles
// check` already reports the full missing-set, so lint only mirrors meaningful
// findings (drift, overlap, register mismatches) once any profile exists.
func lintRoleProfiles() []ctxLintIssue {
	files, err := dirFiles(sdtRolesDir)
	if err != nil || len(files) == 0 {
		return nil
	}
	var issues []ctxLintIssue
	for _, f := range roleCheckFindings() {
		if f.Priority == ctxLintWarning || f.Priority == ctxLintCritical {
			issues = append(issues, ctxLintIssue{Path: f.Path, Priority: ctxLintSuggestion, Message: f.Message, Hint: f.Hint})
		}
	}
	return issues
}

var agentRolesCheckCmd = &cobra.Command{
	Use:   useCheck,
	Short: "Validate the role profiles deterministically",
	Long: `Validate the generated role profiles under
` + "`context/roles/`" + `, no runtime required:

  - closed register + profile presence: the on-disk set must equal the register
    exactly (shared.md + one ` + "`<slug>.md`" + ` per register slug);
  - drift guard: every profile's generated ` + "`core`" + `/` + "`project`" + `
    layers must match a fresh render (user-owned ` + "`preferences`" + ` is never
    treated as drift);
  - owner/reviewer overlap: no path may be claimed by more than one role.

Exits non-zero when any CRITICAL finding exists. Advisory mirrors of these
checks surface in ` + "`sdt context lint`" + ` and ` + "`sdt agent doctor`" + `.

Examples:
  sdt agent roles check
  sdt agent roles check --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		findings := roleCheckFindings()
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(findings, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(findings)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, f := range findings {
				line := fmt.Sprintf("[%s] %s: %s", f.Priority, f.Path, f.Message)
				if f.Hint != "" {
					line += " — hint: " + f.Hint
				}
				outputString(cmd, line+"\n")
			}
		}
		critical := 0
		for _, f := range findings {
			if f.Priority == ctxLintCritical {
				critical++
			}
		}
		if critical > 0 {
			exitWithError(cmd, fmt.Errorf("%d CRITICAL role check finding(s)", critical))
		}
	},
}
