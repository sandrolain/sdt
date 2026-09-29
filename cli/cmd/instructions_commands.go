package cmd

import (
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

// commandPayloadUndeclared is the index cell for a trigger with no declared
// payload. It is explicit rather than blank so a missing declaration is never
// read as "this command accepts no payload".

const commandPayloadUndeclared = "_not declared_"

// commandIndexEntry is one row of the commands registry: the trigger and the
// payload phrase it accepts. The command file and durable contract columns are
// derived from the trigger id itself.

type commandIndexEntry struct {
	Trigger string
	Payload string
}

// declaredCommandPayload returns the payload phrase declared for a trigger in
// agentCommandStubs, or the undeclared placeholder when the trigger is not in
// the table (a user trigger created with `sdt context commands new`).

func declaredCommandPayload(id string) string {
	for _, s := range agentCommandStubs {
		if s.id == id {
			return s.payload
		}
	}
	return commandPayloadUndeclared
}

type commandsIndexTemplateData struct {
	Project  string
	Now      string
	Triggers []commandIndexEntry
}

func instrCommandsIndexTemplate(entries []commandIndexEntry, project string, now time.Time) string {
	return templates.Must("commands/index.md.tmpl", commandsIndexTemplateData{
		Project:  project,
		Now:      now.UTC().Format(time.RFC3339),
		Triggers: entries,
	}, nil)
}

type commandStubTemplateData struct {
	ID       string
	Contract string
	Payload  string
	Examples []string
	Project  string
	Now      string
}

// instrCommandStubTemplate builds the thin command file for one trigger: it
// resolves the trigger and delegates to the durable contract. payload and
// examples describe what the trigger takes after the colon in `>id: payload`;
// an empty payload renders the generic declare-it-here block.

func instrCommandStubTemplate(id, contract, payload string, examples []string, project string, now time.Time) string {
	return templates.Must("commands/stub.md.tmpl", commandStubTemplateData{
		ID:       id,
		Contract: contract,
		Payload:  payload,
		Examples: examples,
		Project:  project,
		Now:      now.UTC().Format(time.RFC3339),
	}, nil)
}
