package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// refcheckDoc is a resolvable context document the dry-run edits.
const refcheckDoc = `---
kind: wiki
id: architecture/refcheck
title: Refcheck
type: concept
status: active
summary: reference check fixture
tags: [test]
---

## Body
`

func writeRefcheckBody(t *testing.T, body string) string {
	t.Helper()
	path := "body.md"
	writeTestFile(t, path, body)
	return path
}

// refCitation renders a citation carrying the artifact's current sha.
func refCitation(t *testing.T, rel, lines string) string {
	t.Helper()
	sha, ok := ctxRefSHA8(filepath.Join(sdtWorkDir, rel))
	if !ok {
		t.Fatalf("fixture artifact %s is missing", rel)
	}
	return rel + "@" + sha + ":" + lines
}

func TestContextEditDryRunAcceptsCleanReferences(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/book.md", claimsArtifact)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", claimsPage)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", refcheckDoc)
	body := writeRefcheckBody(t, "## Body\n\nSee [[architecture/alpha|alpha]] and "+refCitation(t, "refs/converted/book.md", "1-5")+".\n")
	out := string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	if strings.Contains(out, "[WARNING]") {
		t.Errorf("a clean body reported a reference issue:\n%s", out)
	}
}

func TestContextEditDryRunRefusesAStaleSha(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/book.md", claimsArtifact)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", refcheckDoc)
	body := writeRefcheckBody(t, "## Body\n\nSee refs/converted/book.md@deadbeef:1-5.\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	})
}

func TestContextEditDryRunRefusesAMissingCitationTarget(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", refcheckDoc)
	body := writeRefcheckBody(t, "## Body\n\nSee refs/converted/missing.md@abcdef12:1-5.\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	})
}

func TestContextEditDryRunRefusesAnUnresolvedLink(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", refcheckDoc)
	body := writeRefcheckBody(t, "## Body\n\nSee [[architecture/no-such-page|missing]].\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	})
}

func TestContextEditDryRunRefusesAMissingRelationTarget(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", strings.Replace(refcheckDoc,
		"summary: reference check fixture",
		"summary: reference check fixture\nrelations:\n  refers_to:\n    - \"[[architecture/no-such-target|missing]]\"", 1))
	body := writeRefcheckBody(t, "## Body\n\nClean.\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	})
}

func TestContextEditDryRunRefusesAClaimWithoutCitation(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", refcheckDoc)
	body := writeRefcheckBody(t, "## Claims\n\n- A claim. {#claim-1} no citation\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	})
}

func TestContextEditDryRunWritesNothingOnABrokenReference(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/refcheck.md", refcheckDoc)
	body := writeRefcheckBody(t, "## Body\n\nSee [[architecture/no-such-page|missing]].\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, "architecture/refcheck", "--file", body, "--dry-run"))
	})
	content := mustReadFile(t, "context/wiki/architecture/refcheck.md")
	if strings.Contains(content, "no-such-page") {
		t.Error("a dry run with a broken reference wrote the body")
	}
}
