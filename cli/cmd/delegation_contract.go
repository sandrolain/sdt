package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const delegationRoleDir = "context/roles"

// materializeDelegationRole makes the generated role profile and shared rules
// readable from the isolated task workspace, including when they are untracked
// in the principal's starting revision.
func materializeDelegationRole(sourceRoot, taskRoot, role string) error {
	if _, ok := roleLookup(role); !ok {
		return fmt.Errorf("unknown SDT role %q", role)
	}
	source, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	workspace, err := filepath.EvalSymlinks(taskRoot)
	if err != nil {
		return fmt.Errorf("resolve task workspace: %w", err)
	}
	sourceDir := filepath.Join(source, delegationRoleDir)
	targetDir := filepath.Join(workspace, delegationRoleDir)
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return err
	}
	resolvedTargetDir, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		return err
	}
	if !withinRoot(workspace, resolvedTargetDir) {
		return fmt.Errorf("task role directory escapes the workspace")
	}
	for _, name := range []string{"shared.md", role + ".md"} {
		path := filepath.Join(sourceDir, name)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve role reference %s: %w", name, err)
		}
		if !withinRoot(source, resolved) {
			return fmt.Errorf("role reference %s escapes the project root", name)
		}
		if err := copyDelegationReference(resolved, filepath.Join(resolvedTargetDir, name)); err != nil {
			return fmt.Errorf("copy role reference %s: %w", name, err)
		}
	}
	return nil
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func copyDelegationReference(source, target string) error {
	in, err := os.Open(source) //#nosec G304 -- generated role references are selected by the closed role register
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // the copy error is authoritative
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file")
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".role-reference-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // remove only if rename did not consume it
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

type delegationBrief struct {
	Goal              string
	Scope             string
	References        []string
	Constraints       []string
	Acceptance        []string
	Role              string
	RoleProfile       string
	IndependentReview bool
}

// renderDelegationBrief emits only the task contract and role guidance. It has
// no principal-verdict field, so an independent reviewer cannot inherit it.
func renderDelegationBrief(brief delegationBrief) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Goal\n%s\n\n## Scope\n%s\n", brief.Goal, brief.Scope)
	writeMarkdownList(&b, "References", brief.References)
	writeMarkdownList(&b, "Constraints", brief.Constraints)
	writeMarkdownList(&b, "Acceptance", brief.Acceptance)
	fmt.Fprintf(&b, "\n## Role instruction\nWork as the %s role.\n", brief.Role)
	if brief.RoleProfile != "" {
		fmt.Fprintf(&b, "Read the role profile at `%s`.\n", brief.RoleProfile)
	}
	if brief.IndependentReview {
		b.WriteString("\nReview the supplied work independently against the acceptance criteria. Do not rely on a principal verdict.\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderDelegationResult(findings, changes, evidence, blockers, remaining, nextAction string) string {
	var b strings.Builder
	writeMarkdownSection(&b, "Findings", findings)
	writeMarkdownSection(&b, "Changes", changes)
	writeMarkdownSection(&b, "Evidence", evidence)
	writeMarkdownSection(&b, "Blockers", blockers)
	writeMarkdownSection(&b, "Remaining work", remaining)
	writeMarkdownSection(&b, "Next action", nextAction)
	return strings.TrimSpace(b.String()) + "\n"
}

func renderDelegationHandoff(verifiedProgress, residualWork, references, nextAction string) string {
	var b strings.Builder
	writeMarkdownSection(&b, "Verified progress", verifiedProgress)
	writeMarkdownSection(&b, "Residual work", residualWork)
	writeMarkdownSection(&b, "References", references)
	writeMarkdownSection(&b, "Next action", nextAction)
	return strings.TrimSpace(b.String()) + "\n"
}

func writeMarkdownList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n", title)
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
}

func writeMarkdownSection(b *strings.Builder, title, body string) {
	if strings.TrimSpace(body) == "" {
		body = "None reported."
	}
	fmt.Fprintf(b, "## %s\n%s\n\n", title, body)
}
