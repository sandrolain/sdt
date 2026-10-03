package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProjectEnvEndToEnd exercises the load through the CLI entry point the way
// a user sees it: a value written to the project-root .env reaches the process
// environment, an exported value wins, and --no-env disables the step.
func TestProjectEnvEndToEnd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"),
		[]byte("SDT_E2E_ENV=from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	// Through the real entry point, without --no-env.
	os.Unsetenv("SDT_E2E_ENV")
	if _, err := ExecuteByArgs([]string{"env", "get", "SDT_E2E_ENV", "--file", filepath.Join(root, ".env")}, nil); err != nil {
		t.Fatalf("env get failed: %v", err)
	}

	// The walked-up file is discoverable from a nested directory.
	if _, ok := findDotEnvPath(); !ok {
		t.Fatalf("findDotEnvPath did not find the root .env from a nested dir")
	}

	// Shell wins over the file.
	t.Setenv("SDT_E2E_ENV", "from-shell")
	if _, err := ExecuteByArgs([]string{"env", "get", "SDT_E2E_ENV", "--file", filepath.Join(root, ".env")}, nil); err != nil {
		t.Fatalf("env get with shell value failed: %v", err)
	}
	if got := os.Getenv("SDT_E2E_ENV"); got != "from-shell" {
		t.Errorf("SDT_E2E_ENV = %q, want from-shell", got)
	}

	// --no-env must not inject the file value when the shell has none.
	os.Unsetenv("SDT_E2E_ENV")
	if _, err := ExecuteByArgs([]string{"--no-env", "version"}, nil); err != nil {
		t.Fatalf("version with --no-env failed: %v", err)
	}
	if _, present := os.LookupEnv("SDT_E2E_ENV"); present {
		t.Errorf("SDT_E2E_ENV should not be set after --no-env")
	}
}
