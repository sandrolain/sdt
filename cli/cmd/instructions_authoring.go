package cmd

import "github.com/sandrolain/sdt/internal/templates"

// instrAuthoringTemplate renders context/instructions/authoring.md: the contract
// for writing or editing instruction files, agent skills and AGENTS.md rules
// (trigger-not-summary descriptions, word budgets, cross-references,
// form-to-failure and micro-testing).
var instrAuthoringTemplate = templates.Must("instructions/authoring.md.tmpl", nil, nil)
