package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetVersion(t *testing.T) {
	// Save original values
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	SetVersion("1.2.3", "abc1234", "2024-01-01")

	if version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", version)
	}
	if commit != "abc1234" {
		t.Errorf("expected commit abc1234, got %s", commit)
	}
	if date != "2024-01-01" {
		t.Errorf("expected date 2024-01-01, got %s", date)
	}
}

func TestVersionCmd_json(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	defer func() { version, commit, date = origVersion, origCommit, origDate }()
	SetVersion("9.8.7", "def456", "2025-06-01")

	out := execute(t, versionCmd, nil)
	var info versionInfo
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if info.Version != "9.8.7" {
		t.Errorf("expected 9.8.7, got %s", info.Version)
	}
}

func TestVersionCmd_short(t *testing.T) {
	origVersion := version
	defer func() { version = origVersion }()
	SetVersion("3.2.1", "x", "y")

	out := execute(t, versionCmd, nil, "--short")
	if !strings.Contains(string(out), "3.2.1") {
		t.Errorf("expected 3.2.1 in output, got: %s", out)
	}
}

func TestVersionCmd_yaml(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	defer func() { version, commit, date = origVersion, origCommit, origDate }()
	SetVersion("1.0.0", "abc", "2024")

	out := execute(t, versionCmd, nil, "--format", "yaml")
	if !strings.Contains(string(out), "version") {
		t.Errorf("expected 'version' in yaml output, got: %s", out)
	}
}
