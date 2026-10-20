package cmd

import (
	"sort"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

// commandPayloadUndeclared is the registry cell for a trigger with no declared
// payload. It is explicit rather than blank so a missing declaration is never
// read as "this command accepts no payload".

const commandPayloadUndeclared = "_not declared_"

// commandIndexEntry is one row of the commands registry: the trigger, the
// durable contract id it invokes, its kind (document vs workflow), the subject
// type a workflow command resolves (empty for a document command), the payload
// phrase it accepts and a couple of concrete invocations.
//
// There is no per-trigger command file any more: the registry is the single
// lookup surface, so a row is the whole declaration. What used to be a file per
// trigger — and the duplicated prose it carried — is now one table.

type commandIndexEntry struct {
	Trigger  string
	Contract string
	Kind     string
	Subject  string
	Payload  string
	Examples string
}

// commandRowExamples renders a stub's examples as one table cell: each in
// backticks, separated by a line break so the row stays a single table line.
func commandRowExamples(examples []string) string {
	if len(examples) == 0 {
		return "—"
	}
	quoted := make([]string, 0, len(examples))
	for _, ex := range examples {
		quoted = append(quoted, "`"+strings.TrimSpace(ex)+"`")
	}
	return strings.Join(quoted, "<br>")
}

// declaredCommandPayload returns the payload phrase declared for a trigger in
// agentCommandStubs, or the undeclared placeholder when the trigger is not in
// the table (a project trigger added with `sdt context commands new`).

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

// commandEntriesFor renders the built-in triggers a profile selection writes,
// sorted by trigger so the table is stable across runs.
func commandEntriesFor(profiles []string) []commandIndexEntry {
	entries := make([]commandIndexEntry, 0, len(agentCommandStubs))
	for _, s := range agentCommandStubs {
		if !stubSelected(s, profiles) {
			continue
		}
		entries = append(entries, commandIndexEntry{
			Trigger:  s.id,
			Contract: s.contract,
			Kind:     s.kind,
			Subject:  s.subject,
			Payload:  s.payload,
			Examples: commandRowExamples(s.examples),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Trigger < entries[j].Trigger })
	return entries
}

type commandsIndexTemplateData struct {
	Project  string
	Now      string
	Triggers []commandIndexEntry
}

// instrCommandsIndexTemplate renders the registry document from its rows.
func instrCommandsIndexTemplate(entries []commandIndexEntry, project string, now time.Time) string {
	return templates.Must("commands/index.md.tmpl", commandsIndexTemplateData{
		Project:  project,
		Now:      now.UTC().Format(time.RFC3339),
		Triggers: entries,
	}, nil)
}
