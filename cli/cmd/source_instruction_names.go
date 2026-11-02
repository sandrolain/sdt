package cmd

import "strings"

// sourceInstructionFile is one third-party file name an agent runtime resolves
// as instructions. `sdt` archives other people's repositories under
// `context/refs/`, and several runtimes load a file they find on the path they
// are working on — Claude Code and Gemini CLI even load one on demand when a
// tool touches a directory. A third party's `AGENTS.md` therefore arrives with
// authority nobody granted it, which is why the archive step stores it under a
// name no runtime resolves.
//
// The list is the pattern the corpus was migrated with on 2026-11-01, including
// the legacy and neighbouring variants (`AGENT.md`, `.windsurfrules`,
// `.clinerules`, `QWEN.md`); `TestSourceInstructionFilesMatchCorpusRename` binds
// this slice to that pattern so a copy cannot drift.
type sourceInstructionFile struct {
	// name is lowercase: the whole basename, or a suffix when suffix is true.
	name string
	// suffix matches by trailing substring (".mdc", ".instructions.md").
	suffix bool
	// loaded is true when a runtime loads the file by that name alone; false
	// when it is loaded only from a configured skill directory (SKILL.md), which
	// makes it a suggestion rather than a warning.
	loaded bool
}

// sourceInstructionArchiveSuffix is appended to an archived file whose name a
// runtime would resolve, so `AGENTS.md` is stored as `AGENTS.md.source`.
const sourceInstructionArchiveSuffix = ".source"

// sourceInstructionFiles is the closed list of names an agent runtime may load
// as instructions. Extending it is a corpus-wide decision, not a local edit.
var sourceInstructionFiles = []sourceInstructionFile{
	{name: "agents.md", loaded: true},
	{name: "agent.md", loaded: true},
	{name: "agents.override.md", loaded: true},
	{name: "claude.md", loaded: true},
	{name: "claude.local.md", loaded: true},
	{name: "gemini.md", loaded: true},
	{name: "qwen.md", loaded: true},
	{name: "copilot-instructions.md", loaded: true},
	{name: ".cursorrules", loaded: true},
	{name: ".windsurfrules", loaded: true},
	{name: ".clinerules", loaded: true},
	{name: ".instructions.md", suffix: true, loaded: true},
	{name: ".mdc", suffix: true, loaded: true},
	{name: "skill.md", loaded: false},
}

// sourceInstructionConfigDirs are the per-runtime configuration directories an
// agent reads agent/command/rule/skill definitions from.
var sourceInstructionConfigDirs = []string{".claude", ".cursor", ".gemini", ".opencode", ".codex"}

// sourceInstructionFor resolves one basename against the closed list. The match
// is case-insensitive: a runtime on a case-insensitive filesystem resolves
// `agents.md` for `AGENTS.md`, so the rename must too.
func sourceInstructionFor(base string) (sourceInstructionFile, bool) {
	lower := strings.ToLower(strings.TrimSpace(base))
	if lower == "" {
		return sourceInstructionFile{}, false
	}
	for _, f := range sourceInstructionFiles {
		if f.suffix {
			if strings.HasSuffix(lower, f.name) {
				return f, true
			}
			continue
		}
		if lower == f.name {
			return f, true
		}
	}
	return sourceInstructionFile{}, false
}

// isSourceInstructionName reports whether an archived basename would be loaded
// by an agent runtime under that name.
func isSourceInstructionName(base string) bool {
	_, ok := sourceInstructionFor(base)
	return ok
}
