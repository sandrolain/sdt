package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitProjectEnvLoadsFromRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SDT_TEST_INIT_ENV=from-file\n# comment\nexport SDT_TEST_INIT_ENV_EXPORT=exported\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	t.Setenv("SDT_TEST_INIT_ENV", "")
	os.Unsetenv("SDT_TEST_INIT_ENV")
	os.Unsetenv("SDT_TEST_INIT_ENV_EXPORT")

	initProjectEnv(nil)

	if got := os.Getenv("SDT_TEST_INIT_ENV"); got != "from-file" {
		t.Errorf("SDT_TEST_INIT_ENV = %q, want from-file", got)
	}
	if got := os.Getenv("SDT_TEST_INIT_ENV_EXPORT"); got != "exported" {
		t.Errorf("SDT_TEST_INIT_ENV_EXPORT = %q, want exported", got)
	}
}

func TestInitProjectEnvShellWins(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SDT_TEST_INIT_ENV=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	t.Setenv("SDT_TEST_INIT_ENV", "from-shell")

	initProjectEnv(nil)

	if got := os.Getenv("SDT_TEST_INIT_ENV"); got != "from-shell" {
		t.Errorf("SDT_TEST_INIT_ENV = %q, want from-shell (the shell must win)", got)
	}
}

func TestInitProjectEnvMissingFileIsNoOp(t *testing.T) {
	t.Setenv("SDT_TEST_INIT_ENV", "")
	os.Unsetenv("SDT_TEST_INIT_ENV")

	initProjectEnv(nil)

	if _, present := os.LookupEnv("SDT_TEST_INIT_ENV"); present {
		t.Errorf("SDT_TEST_INIT_ENV should not be set when no .env exists")
	}
}

func TestInitProjectEnvNoEnvFlag(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SDT_TEST_INIT_ENV=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	os.Unsetenv("SDT_TEST_INIT_ENV")

	cmd := &cobra.Command{}
	cmd.Flags().Bool("no-env", true, "")
	// Attach the flag to a root command, since getPersistentBool reads the
	// root's persistent flags.
	rootCmd.PersistentFlags().Lookup("no-env").Value.Set("true")
	defer rootCmd.PersistentFlags().Lookup("no-env").Value.Set("false")

	initProjectEnv(rootCmd)

	if _, present := os.LookupEnv("SDT_TEST_INIT_ENV"); present {
		t.Errorf("SDT_TEST_INIT_ENV should not be set with --no-env")
	}
}

func TestInitProjectEnvMalformedLineIsSkipped(t *testing.T) {
	root := t.TempDir()
	content := "NOT A VALID LINE\nSDT_TEST_INIT_ENV=ok\n=novalue\n\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	os.Unsetenv("SDT_TEST_INIT_ENV")

	initProjectEnv(nil)

	if got := os.Getenv("SDT_TEST_INIT_ENV"); got != "ok" {
		t.Errorf("SDT_TEST_INIT_ENV = %q, want ok (malformed lines are skipped)", got)
	}
}
