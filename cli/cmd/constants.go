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

	// Agent names used by the skill command templates.
	agentNameClaude  = "claude"
	agentNameGeneric = "generic"
	agentNameSkill   = "skill"

	// SDT project config file name.
	sdtConfigFile = ".sdt.yaml"

	// sdt.context/ working directory layout created by sdt agent init.
	sdtWorkDir    = "sdt.context"
	sdtPlanDir    = "sdt.context/plan"
	sdtWorklogDir = "sdt.context/worklog"
	sdtNotesDir   = "sdt.context/notes"
	sdtTmpDir     = "sdt.context/tmp"
	sdtWorkReadme = "sdt.context/README.md"

	// Agent section names.
	agentSectionNameProject     = "project"
	agentSectionNameCommands    = "commands"
	agentSectionNameWorkflow    = "workflow"
	agentSectionNameMemory      = "memory"
	agentSectionNamePlanning    = "planning"
	agentSectionNameAnnotations = "annotations"
	agentSectionNameSelfUpdate  = "self-update"

	// File result statuses.
	statusCreated = "created"
	statusSkipped = "skipped"
	statusError   = "error"
	statusWritten = "written"
	statusUpdated = "updated"
	statusDryRun  = "dry-run"

	// Extended guide file names.
	guideFileSkill     = "SKILL.md"
	guideFileReference = "REFERENCE.md"
	guideFileWorkflows = "WORKFLOWS.md"

	// Cobra command Use strings shared across files.
	useInit   = "init"
	useMemory = "memory"
)
