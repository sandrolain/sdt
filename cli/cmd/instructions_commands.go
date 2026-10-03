package cmd

import (
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

// commandPayloadUndeclared is the index cell for a trigger with no declared
// payload. It is explicit rather than blank so a missing declaration is never
// read as "this command accepts no payload".

const commandPayloadUndeclared = "_not declared_"

// commandIndexEntry is one row of the commands registry: the trigger, the
// durable contract id it invokes, its kind (document vs workflow), the subject
// type a workflow command resolves (empty for a document command) and the
// payload phrase it accepts. The command file column is derived from the trigger
// id itself.

type commandIndexEntry struct {
	Trigger  string
	Contract string
	Kind     string
	Subject  string
	Payload  string
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

// declaredCommandContract returns the durable instruction id a trigger invokes
// from agentCommandStubs, or the trigger id itself for an undeclared (user)
// trigger, whose contract defaults to the trigger name.
func declaredCommandContract(id string) string {
	for _, s := range agentCommandStubs {
		if s.id == id {
			return s.contract
		}
	}
	return id
}

// declaredCommandKind returns the declared kind for a trigger, defaulting to a
// document command for a trigger not in the table (a user trigger).
func declaredCommandKind(id string) string {
	for _, s := range agentCommandStubs {
		if s.id == id {
			return s.kind
		}
	}
	return commandKindDocument
}

// declaredCommandSubject returns the declared subject for a trigger, or "" when
// the trigger is not in the table or is a document command.
func declaredCommandSubject(id string) string {
	for _, s := range agentCommandStubs {
		if s.id == id {
			return s.subject
		}
	}
	return ""
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
	Kind     string
	Subject  string
	Workflow bool
	Payload  string
	Examples []string
	Project  string
	Now      string
}

// instrCommandStubTemplate builds the thin command file for one trigger: it
// resolves the trigger and delegates to the durable contract. kind/subject
// declare the taxonomy projection (a workflow command renders a ## Subject
// block; a document command does not). payload and examples describe what the
// trigger takes after the colon in `>id: payload`; an empty payload renders the
// generic declare-it-here block.

func instrCommandStubTemplate(id, contract, kind, subject, payload string, examples []string, project string, now time.Time) string {
	return templates.Must("commands/stub.md.tmpl", commandStubTemplateData{
		ID:       id,
		Contract: contract,
		Kind:     kind,
		Subject:  subject,
		Workflow: kind == commandKindWorkflow,
		Payload:  payload,
		Examples: examples,
		Project:  project,
		Now:      now.UTC().Format(time.RFC3339),
	}, nil)
}
