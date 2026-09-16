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
	sdtWorkDir           = "context"
	readmeFile           = "README.md"
	sdtPlanDir           = "context/plan"
	sdtAnalysisDir       = "context/analysis"
	sdtWorklogDir        = "context/worklog"
	sdtNotesDir          = "context/notes"
	sdtTasksDir          = "context/tasks"
	sdtArchiveDir        = "context/archive"
	sdtTmpDir            = "context/tmp"
	sdtDocsDir           = "context/sdtdocs"
	sdtDocsReadme        = "context/sdtdocs/README.md"
	sdtInstrDir          = "context/instructions"
	sdtCommandsDir       = "context/commands"
	sdtCommandsIndex     = "context/commands/index.md"
	sdtWorkReadme        = "context/README.md"
	sdtContextIndex      = "context/index.md"
	sdtArchitectureDir   = "context/architecture"
	sdtDecisionsDir      = "context/decisions"
	sdtQuestionsDir      = "context/questions"
	sdtProposalsDir      = "context/proposals"
	sdtPromptsDir        = "context/prompts"
	sdtResearchDir       = "context/research"
	sdtScriptsDir        = "context/scripts"
	sdtInstrProject      = "context/instructions/project.md"
	sdtInstrReference    = "context/instructions/reference.md"
	sdtInstrCli          = "context/instructions/cli.md"
	sdtInstrAnalysis     = "context/instructions/analysis.md"
	sdtInstrPlan         = "context/instructions/plan.md"
	sdtInstrTasks        = "context/instructions/tasks.md"
	sdtInstrDecision     = "context/instructions/decision.md"
	sdtInstrArchitecture = "context/instructions/architecture.md"
	sdtInstrWorklog      = "context/instructions/worklog.md"
	sdtInstrNotes        = "context/instructions/notes.md"
	sdtInstrQuestions    = "context/instructions/questions.md"
	sdtInstrProposal     = "context/instructions/proposal.md"
	sdtInstrPrompts      = "context/instructions/prompts.md"
	sdtInstrResearch     = "context/instructions/research.md"
	sdtInstrIngestion    = "context/instructions/ingestion.md"
	sdtInstrScripts      = "context/instructions/scripts.md"
	sdtInstrWiki         = "context/instructions/wiki.md"
	sdtScriptsIndex      = "context/scripts/index.md"
	sdtWikiDir           = "context/wiki"
	sdtIngestionDir      = "context/ingestion"
	sdtRefsDir           = "context/refs"

	// The tagged section names in AGENTS.md.
	agentSectionNameInstructions = "instructions"
	agentSectionNameProject      = "project"

	// File result statuses.
	statusCreated = "created"
	statusSkipped = "skipped"
	statusError   = "error"
	statusWritten = "written"
	statusUpdated = "updated"
	statusRemoved = "removed"
	statusDryRun  = "dry-run"

	// Cobra command Use strings shared across files.
	useInit = "init"
	useList = "list"

	// Context work file task status values.
	taskStatusTodo      = "todo"
	taskStatusDone      = "done"
	taskStatusWip       = "wip"
	taskStatusBlocked   = "blocked"
	taskStatusBlock     = "block"
	ctxFrontmatterDelim = "---"

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
