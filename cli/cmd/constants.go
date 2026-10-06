package cmd

const (
	// Subcommand names used in multiple command files.
	cmdDec     = "dec"
	cmdReplace = "replace"
	cmdVerify  = "verify"
	cmdValid   = "valid"

	// Type identifiers shared by config and schema commands.
	typeString      = "string"
	typeInt         = "int"
	typeStringArray = "stringArray"

	// PEM block type headers.
	pemTypeRSAPrivateKey = "RSA PRIVATE KEY"
	pemTypePrivateKey    = "PRIVATE KEY"
	pemTypePublicKey     = "PUBLIC KEY"

	// SDT project config file name.
	sdtConfigFile = ".sdt.yaml"

	// context/ working directory layout created by sdt agent init.
	sdtWorkDir         = "context"
	readmeFile         = "README.md"
	sdtPlanDir         = "context/plan"
	sdtAnalysisDir     = "context/analysis"
	sdtWorklogDir      = "context/worklog"
	sdtNotesDir        = "context/notes"
	sdtTasksDir        = "context/tasks"
	sdtDeprecatedDir   = "context/deprecated"
	sdtTmpDir          = "context/tmp"
	sdtDocsDir         = "context/sdtdocs"
	sdtDocsReadme      = "context/sdtdocs/README.md"
	sdtInstrDir        = "context/instructions"
	sdtCommandsDir     = "context/commands"
	sdtCommandsIndex   = "context/commands/index.md"
	sdtWorkReadme      = "context/README.md"
	sdtContextIndex    = "context/index.md"
	sdtArchitectureDir = "context/architecture"
	sdtDecisionsDir    = "context/decisions"
	sdtQuestionsDir    = "context/questions"
	sdtProposalsDir    = "context/proposals"
	sdtPromptsDir      = "context/prompts"
	sdtResearchDir     = "context/research"
	sdtScriptsDir      = "context/scripts"
	sdtRolesDir        = "context/roles"
	sdtScriptsIndex    = "context/scripts/index.md"
	sdtWikiDir         = "context/wiki"
	sdtIngestionDir    = "context/ingestion"
	sdtRefsDir         = "context/refs"
	sdtConvertedDir    = "context/refs/converted"

	// The tagged section names in AGENTS.md.
	agentSectionNameInstructions = "instructions"
	agentSectionNameProject      = "project"

	// File result statuses.
	statusCreated  = "created"
	statusSkipped  = "skipped"
	statusError    = "error"
	statusWritten  = "written"
	statusUpdated  = "updated"
	statusRemoved  = "removed"
	statusArchived = "archived"
	statusDryRun   = "dry-run"

	// Document statuses.
	statusPostponed = "postponed"

	// Cobra command Use strings shared across files.
	useInit   = "init"
	useList   = "list"
	useShow   = "show"
	useCheck  = "check"
	cmdDocs   = "docs"
	useDoneID = "done <id>"
	useAgent  = "agent"

	// Role slugs, the closed register namespace. Kept as constants so the same
	// literal is never repeated across register/templates/derivation.
	roleSlugPM        = "pm"
	roleSlugBackend   = "backend"
	roleSlugFrontend  = "frontend"
	roleSlugArchitect = "architect"
	roleSlugReviewer  = "reviewer"
	roleSlugQA        = "qa"
	roleSlugDevops    = "devops"

	// Context work file task status values.
	taskStatusTodo      = "todo"
	taskStatusDone      = "done"
	taskStatusWip       = "wip"
	taskStatusBlocked   = "blocked"
	taskStatusBlock     = "block"
	ctxFrontmatterDelim = "---"

	// Context work file frontmatter keys. These name the same field wherever it
	// is read, written, linted or used as a query term, so a single literal per
	// key keeps the field name from drifting between those call sites.
	ctxFrontmatterKind        = "kind"
	ctxFrontmatterNumber      = "number"
	ctxFrontmatterObjective   = "objective"
	ctxFrontmatterTopics      = "topics"
	ctxFrontmatterAgent       = "agent"
	ctxFrontmatterContradicts = "contradicts"

	// Reverse parent→children list fields complementing ctxKeyAnalysisID and
	// ctxKeyPlanID.
	ctxKeyPlansIDs = "plans_ids"
	ctxKeyTasksIDs = "tasks_ids"

	// ctxBackfillVerb names the one-shot backfill subcommand shared by the uid,
	// relations and checklist command groups.
	ctxBackfillVerb = "backfill"

	// Task FILE frontmatter status values (tasks.md contract). At least three
	// states are always available: pending (to work on), in-progress,
	// completed; archived closes the file. `active` is the legacy value
	// accepted by lint for pre-change task files.
	taskFileStatusPending    = "pending"
	taskFileStatusInProgress = "in-progress"
	taskFileStatusCompleted  = "completed"
	taskFileStatusArchived   = "archived"
	taskFileStatusLegacy     = "active"

	// Canonical string literal for boolean-flag defaults.
	flagValueFalse = "false"
)
