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

	// sdt.context/ working directory layout created by sdt agent init.
	sdtWorkDir        = "sdt.context"
	readmeFile        = "README.md"
	sdtPlanDir        = "sdt.context/plan"
	sdtWorklogDir     = "sdt.context/worklog"
	sdtNotesDir       = "sdt.context/notes"
	sdtTasksDir       = "sdt.context/tasks"
	sdtTasksTODO      = "sdt.context/tasks/TODO.md"
	sdtArchiveDir     = "sdt.context/archive"
	sdtTmpDir         = "sdt.context/tmp"
	sdtDocsDir        = "sdt.context/docs"
	sdtDocsReadme     = "sdt.context/docs/README.md"
	sdtInstrDir       = "sdt.context/instructions"
	sdtWorkReadme     = "sdt.context/README.md"
	sdtInstrProject   = "sdt.context/instructions/project.md"
	sdtInstrMemory    = "sdt.context/instructions/memory.md"
	sdtInstrReference = "sdt.context/instructions/reference.md"
	sdtInstrCli       = "sdt.context/instructions/cli.md"

	// The single tagged section name in AGENTS.md.
	agentSectionNameInstructions = "instructions"

	// File result statuses.
	statusCreated = "created"
	statusSkipped = "skipped"
	statusError   = "error"
	statusWritten = "written"
	statusUpdated = "updated"
	statusRemoved = "removed"
	statusDryRun  = "dry-run"

	// Cobra command Use strings shared across files.
	useInit   = "init"
	useMemory = "memory"
	useList   = "list"

	// Context work file task status values.
	taskStatusTodo      = "todo"
	taskStatusDone      = "done"
	taskStatusWip       = "wip"
	taskStatusBlocked   = "blocked"
	taskStatusBlock     = "block"
	ctxFrontmatterDelim = "---"
)
