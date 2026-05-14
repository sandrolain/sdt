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

	// Agent names used by setup and skill commands.
	agentNameClaude  = "claude"
	agentNameGeneric = "generic"
	agentNameSkill   = "skill"

	// Agent instruction file paths.
	agentFileCopilotMD = ".github/copilot-instructions.md"
	agentFileClaudeMD  = "CLAUDE.md"
	agentFileAgentsMD  = "AGENTS.md"
	agentFileSkillMD   = ".agents/skills/sdt/SKILL.md"
)
