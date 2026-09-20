package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// The expected supported-type sets below mirror the registry (context_types.go)
// and document the CLI surface. A registry change that alters a surface set
// without intent fails these tests; they also guard against re-introducing
// hand-written type lists in commands (stale-drift guard).

const (
	wantNewTypes      = "plan|analysis|worklog|notes|questions|proposal|prompt|research|architecture|decision|wiki"
	wantPathTypes     = "plan|analysis|worklog|notes|questions|proposal|prompt|research|architecture|decision|tasks|tmp|archive|wiki"
	wantListTypes     = "plan|analysis|worklog|notes|questions|proposal|prompt|research|architecture|decisions|tasks|archive|commands"
	wantTemplateTypes = "plan|analysis|worklog|notes|questions|proposal|prompt|research|architecture|decision|tasks|wiki"
)

func TestContextTypeRegistry(t *testing.T) {
	// Every expected kind resolves.
	kinds := []string{
		ctxTypePlan, ctxTypeAnalysis, ctxTypeWorklog, ctxTypeNotes, ctxTypeQuestions,
		ctxTypeProposal, ctxTypePrompt, ctxTypeResearch, ctxTypeArchitecture,
		ctxTypeDecision, ctxTypeTasks, ctxTypeTmp, ctxTypeArchive, ctxTypeCommands, ctxTypeWiki,
	}
	for _, k := range kinds {
		if _, ok := ctxTypeLookup(k); !ok {
			t.Errorf("registry missing kind %q", k)
		}
	}
	if got := len(contextTypeList); got != len(kinds) {
		t.Errorf("registry has %d entries, want %d (unexpected kind added/removed)", got, len(kinds))
	}

	// No registry entry exceeds the known kind set.
	for _, e := range contextTypeList {
		found := false
		for _, k := range kinds {
			if e.kind == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("registry has unknown kind %q", e.kind)
		}
	}

	// No dir is claimed by two types.
	seen := map[string]string{}
	for _, e := range contextTypeList {
		if prev, ok := seen[e.dir]; ok {
			t.Errorf("dir %q claimed by both %q and %q", e.dir, prev, e.kind)
		}
		seen[e.dir] = e.kind
	}

	if _, ok := ctxTypeLookup("bogus"); ok {
		t.Error("ctxTypeLookup resolved an unknown kind")
	}
	if _, ok := ctxTypeForDir("context/bogus"); ok {
		t.Error("ctxTypeForDir resolved an unknown dir")
	}
}

// TestContextTypeDirTier covers the dir/tier roundtrip and the reindex tier
// mapping. A "" registry tier means "not part of the reindexed tree", for which
// ctxTierForDir falls back to history.

func TestContextTypeDirTier(t *testing.T) {
	cases := []struct {
		kind   string
		dir    string
		tier   string
		hasUpd bool
		alias  string
	}{
		{ctxTypePlan, sdtPlanDir, "medium", true, ""},
		{ctxTypeAnalysis, sdtAnalysisDir, ctxTierImportant, true, ""},
		{ctxTypeWorklog, sdtWorklogDir, ctxTierHistory, true, ""},
		{ctxTypeNotes, sdtNotesDir, "medium", false, ""},
		{ctxTypeQuestions, sdtQuestionsDir, "medium", true, ""},
		{ctxTypeProposal, sdtProposalsDir, ctxTierImportant, true, ""},
		{ctxTypePrompt, sdtPromptsDir, "medium", true, ""},
		{ctxTypeResearch, sdtResearchDir, ctxTierImportant, true, ""},
		{ctxTypeArchitecture, sdtArchitectureDir, "essential", true, ""},
		{ctxTypeDecision, sdtDecisionsDir, "essential", false, "decisions"},
		{ctxTypeTasks, sdtTasksDir, "operational", true, ""},
		{ctxTypeTmp, sdtTmpDir, "", false, ""},
		{ctxTypeArchive, sdtArchiveDir, ctxTierHistory, false, ""},
		{ctxTypeCommands, sdtCommandsDir, "operational", false, ""},
		{ctxTypeWiki, sdtWikiDir, "", true, ""},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			tp, ok := ctxTypeLookup(c.kind)
			if !ok {
				t.Fatalf("registry missing %q", c.kind)
			}
			if got, _ := contextDir(c.kind); got != c.dir {
				t.Errorf("contextDir = %q, want %q", got, c.dir)
			}
			if got, ok := ctxTypeForDir(c.dir); !ok || got.kind != c.kind {
				t.Errorf("ctxTypeForDir(%q) = %q@%v, want %q", c.dir, got.kind, ok, c.kind)
			}
			if got := ctxHasUpdatedFor(c.kind); got != c.hasUpd {
				t.Errorf("ctxHasUpdatedFor = %v, want %v", got, c.hasUpd)
			}
			labelWant := c.kind
			if c.alias != "" {
				labelWant = c.alias
			}
			if got := ctxKindLabel(tp); got != labelWant {
				t.Errorf("ctxKindLabel = %q, want %q", got, labelWant)
			}
			tierWant := c.tier
			if c.tier == "" {
				tierWant = ctxTierHistory
			}
			if got := ctxTierForDir(c.dir); got != tierWant {
				t.Errorf("ctxTierForDir(%q) = %q, want %q", c.dir, got, tierWant)
			}
		})
	}
	// Unknown dirs fall back to history (reindex grouping).
	if got := ctxTierForDir("context/unlisted"); got != ctxTierHistory {
		t.Errorf("unknown dir tier = %q, want %q", got, ctxTierHistory)
	}
}

func TestContextTypeStatuses(t *testing.T) {
	cases := []struct {
		kind   string
		defSt  string
		status string // pipe-joined vocabulary
	}{
		{ctxTypePlan, ctxWikiStatusActive, "active|completed|abandoned"},
		{ctxTypeAnalysis, ctxWikiStatusActive, "active|draft|archived"},
		{ctxTypeQuestions, ctxWikiStatusActive, "active|resolved"},
		{ctxTypeProposal, ctxWikiStatusDraft, "draft|review|accepted|rejected|superseded"},
		{ctxTypePrompt, ctxWikiStatusDraft, "draft|active|archived"},
		{ctxTypeResearch, ctxWikiStatusDraft, "draft|active|archived"},
		{ctxTypeArchitecture, ctxWikiStatusDraft, "draft|current|superseded"},
		{ctxTypeDecision, "proposed", "proposed|accepted|rejected|deprecated|superseded"},
		{ctxTypeTasks, taskFileStatusPending, "pending|in-progress|completed|archived|active"},
		{ctxTypeArchive, taskFileStatusArchived, "archived"},
		{ctxTypeCommands, ctxWikiStatusActive, "active"},
		{ctxTypeWiki, ctxWikiStatusDraft, "draft|active|archived"},
		{ctxTypeWorklog, "", ""},
		{ctxTypeNotes, "", ""},
		{ctxTypeTmp, "", ""},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			tp, ok := ctxTypeLookup(c.kind)
			if !ok {
				t.Fatalf("registry missing %q", c.kind)
			}
			if got, has := ctxDefaultStatusFor(c.kind); has != (c.defSt != "") || (c.defSt != "" && got != c.defSt) {
				t.Errorf("ctxDefaultStatusFor = %q@%v, want %q", got, has, c.defSt)
			}
			if tp.defaultStatus != c.defSt {
				t.Errorf("defaultStatus = %q, want %q", tp.defaultStatus, c.defSt)
			}
			if got := strings.Join(tp.statuses, "|"); got != c.status {
				t.Errorf("statuses = %s, want %s", got, c.status)
			}
		})
	}
}

func TestContextTypeHelpTexts(t *testing.T) {
	if got := ctxTypeHelpText(ctxNewTypes()); got != wantNewTypes {
		t.Errorf("new types = %s, want %s", got, wantNewTypes)
	}
	if got := ctxTypeHelpText(ctxPathTypes()); got != wantPathTypes {
		t.Errorf("path types = %s, want %s", got, wantPathTypes)
	}
	if got := ctxListHelpText(); got != wantListTypes {
		t.Errorf("list types = %s, want %s", got, wantListTypes)
	}
	if got := ctxTypeHelpText(ctxTemplateTypes()); got != wantTemplateTypes {
		t.Errorf("template types = %s, want %s", got, wantTemplateTypes)
	}
}

// TestContextTypeFlagHelpParity guards the user-visible --type help strings:
// they must be derived from the registry (context_types.go), never from a
// hand-written literal that can drift.

func TestContextTypeFlagHelpParity(t *testing.T) {
	for _, c := range []struct {
		cmd  *cobra.Command
		want string
	}{
		{contextPathCmd, "Type: " + wantPathTypes},
		{contextNewCmd, "Type: " + wantNewTypes},
		{contextListCmd, "Type: " + wantListTypes},
		{contextTemplateCmd, "Type: " + wantTemplateTypes},
	} {
		t.Run(c.cmd.Use, func(t *testing.T) {
			f := c.cmd.Flags().Lookup("type")
			if f == nil {
				t.Fatal("missing --type flag")
			}
			if got := f.Usage; got != c.want {
				t.Errorf("--type help = %q, want %q", got, c.want)
			}
		})
	}
}

// TestContextPathRegistryScheme verifies path derivation stays correct through
// the registry schemes with a pinned clock.

func TestContextPathRegistryScheme(t *testing.T) {
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	cases := []struct {
		kind, slug string
		want       string
	}{
		{ctxTypePlan, "ship-memory", filepath.Join(sdtPlanDir, "20260806-070000-ship-memory.md")},
		{ctxTypeQuestions, "open-api", filepath.Join(sdtQuestionsDir, "20260806-070000-open-api.md")},
		{ctxTypeProposal, "provenance", filepath.Join(sdtProposalsDir, "20260806-070000-provenance.md")},
		{ctxTypeResearch, "backends", filepath.Join(sdtResearchDir, "20260806-070000-backends.md")},
		{ctxTypeArchive, "old-plan", filepath.Join(sdtArchiveDir, "20260806-070000-old-plan.md")},
		{ctxTypeArchitecture, "config-loading", filepath.Join(sdtArchitectureDir, "config-loading.md")},
		{ctxTypeWiki, "backend/auth", filepath.Join(sdtWikiDir, "backend", "auth.md")},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			runInTempDir(t)
			got, err := contextPath(c.kind, c.slug, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("contextPath = %q, want %q", got, c.want)
			}
		})
	}

	if _, err := contextPath(ctxTypeDecision, "auth", "", ""); err == nil {
		t.Error("decision path must error (append-only scheme)")
	}
	if _, err := contextPath(ctxTypeTasks, "t", "", "plan-ref"); err == nil {
		t.Error("tasks path without --phase must error")
	}
	if _, err := contextPath(ctxTypeTmp, "", "", ""); err == nil {
		t.Error("tmp path without --slug must error")
	}
	if _, err := contextPath("commands", "x", "", ""); err == nil {
		t.Error("commands is not path-supported; must error")
	}
}

func TestContextInstrPathRegistry(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{ctxTypeAnalysis, filepath.Join(sdtInstrDir, "analysis.md")},
		{ctxTypePrompt, filepath.Join(sdtInstrDir, "prompts.md")},
		{ctxTypeResearch, filepath.Join(sdtInstrDir, "research.md")},
	}
	for _, c := range cases {
		got, err := contextInstrPath(c.kind)
		if err != nil {
			t.Fatalf("%s: %v", c.kind, err)
		}
		if got != c.want {
			t.Errorf("contextInstrPath(%s) = %q, want %q", c.kind, got, c.want)
		}
	}
	for _, kind := range []string{ctxTypeCommands, ctxTypeTmp} {
		if _, err := contextInstrPath(kind); err == nil {
			t.Errorf("contextInstrPath(%s) must error (template not allowed)", kind)
		}
	}
	if got, err := contextInstrPath(ctxTypeWiki); err != nil {
		t.Errorf("contextInstrPath(wiki) = %v (wiki is template-allowed)", err)
	} else if got != filepath.Join(sdtInstrDir, "wiki.md") {
		t.Errorf("contextInstrPath(wiki) = %q", got)
	}
}

func TestStatusRowsRegistryLabels(t *testing.T) {
	rows := ctxStatusRows()
	labels := map[string]bool{}
	for _, r := range rows {
		labels[r.Type] = true
	}
	for _, want := range []string{"architecture", "decisions", "analysis", "plan", "notes", "proposal", "prompt", "research", "questions", "tasks", "commands", "worklog", "archive"} {
		if !labels[want] {
			t.Errorf("status summary missing row %q", want)
		}
	}
	if labels["tmp"] || labels["wiki"] {
		t.Errorf("status summary must not include tmp/wiki rows")
	}
}
