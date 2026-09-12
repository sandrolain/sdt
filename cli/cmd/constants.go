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
	sdtDocsDir           = "context/docs"
	sdtDocsReadme        = "context/docs/README.md"
	sdtInstrDir          = "context/instructions"
	sdtWorkReadme        = "context/README.md"
	sdtContextIndex      = "context/index.md"
	sdtArchitectureDir   = "context/architecture"
	sdtDecisionsDir      = "context/decisions"
	sdtQuestionsDir      = "context/questions"
	sdtScriptsDir        = "context/scripts"
	sdtInstrProject      = "context/instructions/project.md"
	sdtInstrReference    = "context/instructions/reference.md"
	sdtInstrCli          = "context/instructions/cli.md"
	sdtInstrAnalysis     = "context/instructions/analysis.md"
	sdtInstrPlan         = "context/instructions/plan.md"
	sdtInstrTasks        = "context/instructions/tasks.md"
	sdtInstrAdr          = "context/instructions/adr.md"
	sdtInstrArchitecture = "context/instructions/architecture.md"
	sdtInstrWorklog      = "context/instructions/worklog.md"
	sdtInstrNotes        = "context/instructions/notes.md"
	sdtInstrQuestions    = "context/instructions/questions.md"
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

	// Canonical string literal for boolean-flag defaults.
	flagValueFalse = "false"
)
