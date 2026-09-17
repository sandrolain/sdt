package cmd

import (
	"fmt"
	"strings"
)

func instrProjectTemplate(project, group string) string {
	var b strings.Builder
	b.WriteString(`# Project
`)
	if project != "" {
		fmt.Fprintf(&b, "\n- Project: %s\n", project)
	}
	if group != "" {
		fmt.Fprintf(&b, "- Group: %s\n", group)
	}
	b.WriteString(`
This project is managed with **SDT** (Smart Developer Tools), a CLI toolset for
AI agents. Every command is deterministic, composable and has machine-readable
output.

## Identity

Project-scoped commands resolve project/group from:

1. ` + "`--project`" + ` / ` + "`--group`" + ` flags
2. ` + "`.sdt.yaml`" + ` found by walking up from the current directory (like ` + "`.git`" + `)
3. otherwise a descriptive error (no implicit fallback)

Without explicit flags the default identity is ` + "`<dirname>_<short-path-hash>`" + `.

## Discovering capabilities

` + codeFence + `
sdt manifest --format json
sdt schema --command "<command>"
` + codeFence + `

## Project-specific conventions

AGENTS.md carries a write-once ` + "`<!-- sdt:begin:project -->`" + ` block with a
single generic template (Stack, Build & Run, Test, Lint & Format, Conventions).
This file is the companion: record the concrete build/test/lint commands and
project conventions here, and keep the AGENTS.md project block in sync when a
pattern becomes a stable convention.
`)
	return b.String()
}
