package cmd

import (
	"errors"
	"testing"
)

func TestCLIDiagnosticPreservesCauseAndValidatesRemedyCommand(t *testing.T) {
	if !commandReferenceExists(rootCmd, "sdt config init") {
		t.Fatal("known remedy command was rejected")
	}
	if commandReferenceExists(rootCmd, "sdt config nonexistent") {
		t.Fatal("unknown remedy command was accepted")
	}
	original := errors.New("missing project config")
	err := withCLIDiagnostic(original, "project config is absent", "initialize the project", "sdt config init")
	var diagnostic *cliDiagnosticError
	if !errors.As(err, &diagnostic) || diagnostic.cause != "project config is absent" || diagnostic.command != "sdt config init" || !errors.Is(err, original) {
		t.Fatalf("diagnostic did not preserve cause, command or wrapped error: %#v", err)
	}
	attrs := cliDiagnosticAttrs(rootCmd, err)
	if got := diagnosticAttr(attrs, "cause"); got != "project config is absent" {
		t.Fatalf("structured cause = %v", got)
	}
	if got := diagnosticAttr(attrs, "remedy_command"); got != "sdt config init" {
		t.Fatalf("validated remedy command = %v", got)
	}
	withoutCommand := withCLIDiagnostic(original, "cause", "try the configured check again", "")
	if got := diagnosticAttr(cliDiagnosticAttrs(rootCmd, withoutCommand), "remedy_command"); got != nil {
		t.Fatalf("absent remedy command was emitted: %v", got)
	}
	withUnknownCommand := withCLIDiagnostic(original, "cause", "remedy", "sdt nonexistent")
	if got := diagnosticAttr(cliDiagnosticAttrs(rootCmd, withUnknownCommand), "remedy_command"); got != nil {
		t.Fatalf("unknown remedy command was emitted: %v", got)
	}
}

func diagnosticAttr(attrs []any, key string) any {
	for i := 0; i+1 < len(attrs); i += 2 {
		if attrs[i] == key {
			return attrs[i+1]
		}
	}
	return nil
}
