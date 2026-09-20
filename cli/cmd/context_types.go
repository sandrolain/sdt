package cmd

import "strings"

// ── shared document-type registry ──────────────────────────────────────────────
//
// ctxDocType is the single source of truth for a context/ work-file type: where
// its files live, the path scheme they follow, its status vocabulary, whether
// the instruction contract requires `updated`, and which CLI surfaces expose
// it. Consumers derive dir, tier, default status, `updated` behavior, template
// file and the --type help lists from this registry; no duplicate map literals.

type ctxPathScheme int

const (
	ctxSchemeDated     ctxPathScheme = iota // <YYYYMMDD-HHMMSS>-<slug>.md
	ctxSchemeBare                           // <slug>.md
	ctxSchemePhase                          // tasks: taskFileFor(phase, plan)
	ctxSchemeTmpBySlug                      // tmp: <slug> under tmp/ (no extension)
	ctxSchemeDecision                       // decisions: append-only via `context new`
	ctxSchemeSubpath                        // wiki: <slug-or-subpath>.md under wiki/
)

type ctxDocType struct {
	kind   string
	dir    string
	tier   string // relevance tier; "" when not part of the reindexed tree
	scheme ctxPathScheme

	defaultStatus string   // initial `status` written by new; "" when the type carries none
	statuses      []string // per-type status vocabulary
	hasUpdated    bool     // instruction contract requires `updated`

	templateFile    string // file name under context/instructions/, "" when none
	newSupported    bool   // `context new --type`
	pathSupported   bool   // `context path --type`
	listSupported   bool   // `context list --type`
	templateAllowed bool   // `context template --type`
	statusRow       bool   // row in `context status`
	alias           string // accepted --type alias (list), e.g. "decisions"
}

// ctxDocStatusSuperseded is the shared vocabulary word used by proposal,
// architecture and decision status lists.

const ctxDocStatusSuperseded = "superseded"

// ctxDocAliasDecision is the accepted --type alias for decision.

const ctxDocAliasDecision = "decisions"

// contextTypeList lists every context/ work-file type in the stable order used
// for the --type help strings.

var contextTypeList = []ctxDocType{
	{kind: ctxTypePlan, dir: sdtPlanDir, tier: ctxTierMedium, scheme: ctxSchemeDated, defaultStatus: ctxWikiStatusActive, statuses: []string{taskFileStatusLegacy, "completed", "abandoned"}, hasUpdated: true, templateFile: "plan.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeAnalysis, dir: sdtAnalysisDir, tier: ctxTierImportant, scheme: ctxSchemeDated, defaultStatus: ctxWikiStatusActive, statuses: []string{taskFileStatusLegacy, ctxWikiStatusDraft, statusArchived}, hasUpdated: true, templateFile: "analysis.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeWorklog, dir: sdtWorklogDir, tier: ctxTierHistory, scheme: ctxSchemeDated, hasUpdated: true, templateFile: "worklog.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeNotes, dir: sdtNotesDir, tier: ctxTierMedium, scheme: ctxSchemeDated, hasUpdated: false, templateFile: "notes.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeQuestions, dir: sdtQuestionsDir, tier: ctxTierMedium, scheme: ctxSchemeDated, defaultStatus: ctxWikiStatusActive, statuses: []string{taskFileStatusLegacy, "resolved"}, hasUpdated: true, templateFile: "questions.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeProposal, dir: sdtProposalsDir, tier: ctxTierImportant, scheme: ctxSchemeDated, defaultStatus: ctxWikiStatusDraft, statuses: []string{ctxWikiStatusDraft, "review", "accepted", "rejected", ctxDocStatusSuperseded}, hasUpdated: true, templateFile: "proposal.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypePrompt, dir: sdtPromptsDir, tier: ctxTierMedium, scheme: ctxSchemeDated, defaultStatus: ctxWikiStatusDraft, statuses: []string{ctxWikiStatusDraft, taskFileStatusLegacy, statusArchived}, hasUpdated: true, templateFile: "prompts.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeResearch, dir: sdtResearchDir, tier: ctxTierImportant, scheme: ctxSchemeDated, defaultStatus: ctxWikiStatusDraft, statuses: []string{ctxWikiStatusDraft, taskFileStatusLegacy, statusArchived}, hasUpdated: true, templateFile: "research.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeArchitecture, dir: sdtArchitectureDir, tier: ctxTierEssential, scheme: ctxSchemeBare, defaultStatus: ctxWikiStatusDraft, statuses: []string{ctxWikiStatusDraft, "current", ctxDocStatusSuperseded}, hasUpdated: true, templateFile: "architecture.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeDecision, dir: sdtDecisionsDir, tier: ctxTierEssential, scheme: ctxSchemeDecision, defaultStatus: "proposed", statuses: []string{"proposed", "accepted", "rejected", "deprecated", ctxDocStatusSuperseded}, hasUpdated: false, templateFile: "decision.md", newSupported: true, pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true, alias: ctxDocAliasDecision},
	{kind: ctxTypeTasks, dir: sdtTasksDir, tier: ctxTierOperational, scheme: ctxSchemePhase, defaultStatus: taskFileStatusPending, statuses: []string{"pending", "in-progress", "completed", taskFileStatusArchived, taskFileStatusLegacy}, hasUpdated: true, templateFile: "tasks.md", pathSupported: true, listSupported: true, templateAllowed: true, statusRow: true},
	{kind: ctxTypeTmp, dir: sdtTmpDir, tier: "", scheme: ctxSchemeTmpBySlug, pathSupported: true, statusRow: false},
	{kind: ctxTypeArchive, dir: sdtArchiveDir, tier: ctxTierHistory, scheme: ctxSchemeDated, defaultStatus: taskFileStatusArchived, statuses: []string{taskFileStatusArchived}, pathSupported: true, listSupported: true, statusRow: true},
	{kind: ctxTypeCommands, dir: sdtCommandsDir, tier: ctxTierOperational, scheme: ctxSchemeBare, defaultStatus: ctxWikiStatusActive, statuses: []string{taskFileStatusLegacy}, listSupported: true, statusRow: true},
	{kind: ctxTypeWiki, dir: sdtWikiDir, tier: "", scheme: ctxSchemeSubpath, defaultStatus: ctxWikiStatusDraft, statuses: []string{ctxWikiStatusDraft, taskFileStatusLegacy, statusArchived}, hasUpdated: true, templateFile: "wiki.md", newSupported: true, pathSupported: true, templateAllowed: true, statusRow: false},
}

func ctxTypeLookup(kind string) (ctxDocType, bool) {
	for _, t := range contextTypeList {
		if t.kind == kind {
			return t, true
		}
	}
	return ctxDocType{}, false
}

func ctxTypeForDir(dir string) (ctxDocType, bool) {
	for _, t := range contextTypeList {
		if t.dir == dir {
			return t, true
		}
	}
	return ctxDocType{}, false
}

// ctxTypeFilter returns the registry entries whose surface flag is set.

func ctxTypesWith(pred func(ctxDocType) bool) []ctxDocType {
	var out []ctxDocType
	for _, t := range contextTypeList {
		if pred(t) {
			out = append(out, t)
		}
	}
	return out
}

func ctxNewTypes() []ctxDocType {
	return ctxTypesWith(func(t ctxDocType) bool { return t.newSupported })
}
func ctxPathTypes() []ctxDocType {
	return ctxTypesWith(func(t ctxDocType) bool { return t.pathSupported })
}
func ctxListTypes() []ctxDocType {
	return ctxTypesWith(func(t ctxDocType) bool { return t.listSupported })
}
func ctxTemplateTypes() []ctxDocType {
	return ctxTypesWith(func(t ctxDocType) bool { return t.templateAllowed })
}

// ctxKindLabel returns the kind, using the accepted --type alias when set
// (e.g. "decisions" for decision).

func ctxKindLabel(t ctxDocType) string {
	if t.alias != "" {
		return t.alias
	}
	return t.kind
}

// ctxTypeHelpText renders the pipe-joined "a|b|c" list used by --type help
// strings and error messages. The plain kind name is used; aliases apply only
// to the per-surface label (see ctxListHelpText).

func ctxTypeHelpText(types []ctxDocType) string {
	labels := make([]string, 0, len(types))
	for _, t := range types {
		labels = append(labels, t.kind)
	}
	return strings.Join(labels, "|")
}

// ctxListHelpText renders the --type list for `context list`, using the
// accepted alias for decision ("decisions").

func ctxListHelpText() string {
	labels := make([]string, 0, len(ctxListTypes()))
	for _, t := range ctxListTypes() {
		labels = append(labels, ctxKindLabel(t))
	}
	return strings.Join(labels, "|")
}

func ctxDefaultStatusFor(kind string) (string, bool) {
	t, ok := ctxTypeLookup(kind)
	if !ok || t.defaultStatus == "" {
		return "", false
	}
	return t.defaultStatus, true
}

func ctxHasUpdatedFor(kind string) bool {
	t, ok := ctxTypeLookup(kind)
	return ok && t.hasUpdated
}
