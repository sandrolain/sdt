package cmd

import "github.com/sandrolain/sdt/internal/templates"

var instrDevelopmentTemplate = templates.Must("instructions/development.md.tmpl", nil, nil)

// instrCommandsIndexTemplate builds context/commands/index.md: the lookup
// surface mapping each trigger to its command file and durable instruction.
// The trigger set comes from the caller so user-created triggers stay listed.
