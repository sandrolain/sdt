package cmd

import (
	"os"
	"strings"
	"testing"
)

const (
	phase3AnalysisDoc     = "context/analysis/20260920-130000-backend.md"
	phase3AnalysisNewSlug = "backend2"
	phase3AnalysisNew     = "context/analysis/20260920-130000-backend2.md"
	phase3AnalysisNewBase = "20260920-130000-backend2"
	phase3RefDoc          = "context/wiki/ref.md"
	phase3WikiDoc         = "context/wiki/backend/auth.md"
	phase3ArchDoc         = "context/architecture/config.md"
	phase3DecisionDoc     = "context/decisions/0001-auth.md"
	phase3PlanDoc         = "context/plan/20260920-131900-plan-x.md"
	phase3TaskDoc         = "context/tasks/20260920-100000-myplan-phase-1.md"
	phase3RefContent      = "---\nkind: wiki\nsummary: \"ref\"\nstatus: active\ncreated: \"2026-09-20T10:00:00Z\"\nlinks:\n  - analysis/20260920-130000-backend.md\nsources:\n  - \"../analysis/20260920-130000-backend.md\"\n---\n\n## Summary\n\nReferencing [[analysis/20260920-130000-backend.md|target]] and [[../analysis/20260920-130000-backend.md]].\n"
	phase3RefNew          = "backend2"
)

func buildPhase3Fixtures(t *testing.T) {
	t.Helper()
	fixtureDoc(t, phase3AnalysisDoc, "analysis", "active", "")
	writeTestFile(t, phase3RefDoc, phase3RefContent)
	writeTestFile(t, "context/index.md", "---\nkind: index\nsummary: \"generated\"\n---\n\n- [[analysis/20260920-130000-backend.md]] — something\n")
	fixtureDoc(t, phase3WikiDoc, "wiki", "active", "")
	fixtureDoc(t, phase3ArchDoc, "architecture", "draft", "")
	fixtureDoc(t, phase3DecisionDoc, "decision", "proposed", "")
	fixtureDoc(t, phase3PlanDoc, "plan", "active", "")
	fixtureDoc(t, phase3TaskDoc, "tasks", "pending", "")
	writeTestFile(t, "context/tmp/scratch.txt", "scratch\n")
}

func TestContextRename(t *testing.T) {
	runInTempDir(t)
	buildPhase3Fixtures(t)

	out := string(execute(t, contextRenameCmd, nil, phase3AnalysisDoc, "--slug", phase3AnalysisNewSlug))
	if !strings.HasPrefix(out, phase3AnalysisNew+"\n") {
		t.Fatalf("rename output missing new path: %q", out)
	}
	if !strings.Contains(out, "rewrote "+phase3RefDoc) {
		t.Errorf("affected file missing from output: %q", out)
	}
	if !strings.Contains(out, "rewrote context/index.md") {
		t.Errorf("index.md missing from output: %q", out)
	}

	if _, err := os.Stat(phase3AnalysisDoc); !os.IsNotExist(err) {
		t.Errorf("old file still exists: %v", err)
	}
	renamed := mustReadFile(t, phase3AnalysisNew)
	if got := frontmatterField(renamed, "summary"); got != `"fixture"` {
		t.Errorf("renamed doc summary = %q", got)
	}

	ref := mustReadFile(t, phase3RefDoc)
	if !strings.Contains(ref, "- analysis/"+phase3AnalysisNewBase+".md") {
		t.Errorf("root-relative link not rewritten:\n%s", ref)
	}
	if !strings.Contains(ref, "- \"../analysis/"+phase3AnalysisNewBase+".md\"") {
		t.Errorf("doc-relative source not rewritten:\n%s", ref)
	}
	if !strings.Contains(ref, "[[analysis/"+phase3AnalysisNewBase+".md|target]]") {
		t.Errorf("wikilink with title not rewritten:\n%s", ref)
	}
	if !strings.Contains(ref, "[[../analysis/"+phase3AnalysisNewBase+".md]]") {
		t.Errorf("doc-relative wikilink not rewritten:\n%s", ref)
	}
	idx := mustReadFile(t, "context/index.md")
	if !strings.Contains(idx, "- [[analysis/"+phase3AnalysisNewBase+".md]]") {
		t.Errorf("index wikilink not rewritten:\n%s", idx)
	}
}

func TestContextRenameBareAndWiki(t *testing.T) {
	runInTempDir(t)
	buildPhase3Fixtures(t)

	out := string(execute(t, contextRenameCmd, nil, phase3ArchDoc, "--slug", "domain-model"))
	if !strings.HasPrefix(out, "context/architecture/domain-model.md\n") {
		t.Fatalf("unexpected bare rename: %q", out)
	}
	if _, err := os.Stat(phase3ArchDoc); !os.IsNotExist(err) {
		t.Error("old architecture file still exists")
	}

	out = string(execute(t, contextRenameCmd, nil, phase3WikiDoc, "--slug", "backend/session"))
	if !strings.HasPrefix(out, "context/wiki/backend/session.md\n") {
		t.Fatalf("unexpected wiki subpath rename: %q", out)
	}
	if _, err := os.Stat(phase3WikiDoc); !os.IsNotExist(err) {
		t.Error("old wiki file still exists")
	}
}

func TestContextRenameRejects(t *testing.T) {
	runInTempDir(t)
	buildPhase3Fixtures(t)

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRenameCmd, nil, phase3DecisionDoc, "--slug", "other"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRenameCmd, nil, phase3TaskDoc, "--slug", "renamed"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRenameCmd, nil, phase3AnalysisDoc))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRenameCmd, nil, phase3AnalysisDoc, "--slug", "backend"))
	})
}

func TestContextRenameDryRun(t *testing.T) {
	runInTempDir(t)
	buildPhase3Fixtures(t)

	out := string(execute(t, contextRenameCmd, nil, phase3AnalysisDoc, "--slug", phase3AnalysisNewSlug, "--dry-run"))
	if !strings.Contains(out, "would rename "+phase3AnalysisDoc+" -> "+phase3AnalysisNew) {
		t.Fatalf("missing dry-run line: %q", out)
	}
	if !strings.Contains(out, "would rewrite "+phase3RefDoc) {
		t.Errorf("missing would-rewrite: %q", out)
	}
	if _, err := os.Stat(phase3AnalysisDoc); err != nil {
		t.Errorf("dry-run must not move the source: %v", err)
	}
	if _, err := os.Stat(phase3AnalysisNew); !os.IsNotExist(err) {
		t.Error("dry-run must not create the target")
	}
	ref := mustReadFile(t, phase3RefDoc)
	if strings.Contains(ref, phase3AnalysisNew) {
		t.Error("dry-run must not rewrite references")
	}
}

func TestContextRenameTargetExists(t *testing.T) {
	runInTempDir(t)
	buildPhase3Fixtures(t)
	// duplicate fixture under the target name keeps it occupied
	fixtureDoc(t, phase3AnalysisNew, "analysis", "active", "")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRenameCmd, nil, phase3AnalysisDoc, "--slug", phase3AnalysisNewSlug))
	})
	if _, err := os.Stat(phase3AnalysisDoc); err != nil {
		t.Errorf("source must survive a blocked rename: %v", err)
	}
}

func TestContextRenameIdempotentReq(t *testing.T) {
	runInTempDir(t)
	buildPhase3Fixtures(t)
	execute(t, contextRenameCmd, nil, phase3AnalysisDoc, "--slug", phase3AnalysisNewSlug)
	// re-running against the (now stale) old reference is a clean error
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRenameCmd, nil, phase3AnalysisDoc, "--slug", "again"))
	})
}
