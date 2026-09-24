package cmd

import (
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

type commandsIndexTemplateData struct {
	Project  string
	Now      string
	Triggers []string
}

func instrCommandsIndexTemplate(ids []string, project string, now time.Time) string {
	return templates.Must("commands/index.md.tmpl", commandsIndexTemplateData{
		Project:  project,
		Now:      now.UTC().Format(time.RFC3339),
		Triggers: ids,
	}, nil)
}

type commandStubTemplateData struct {
	ID       string
	Contract string
	Project  string
	Now      string
}

// instrCommandStubTemplate builds the thin command file for one trigger: it
// resolves the trigger and delegates to the durable contract.

func instrCommandStubTemplate(id, contract, project string, now time.Time) string {
	return templates.Must("commands/stub.md.tmpl", commandStubTemplateData{
		ID:       id,
		Contract: contract,
		Project:  project,
		Now:      now.UTC().Format(time.RFC3339),
	}, nil)
}
