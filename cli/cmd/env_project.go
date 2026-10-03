package cmd

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// initProjectEnv loads a project-root .env file into the process environment
// before the command body runs. It walks up from the current working directory
// looking for .env, the way findProjectConfig looks for .sdt.yaml, and sets
// only the variables that are not already present: a value exported by the
// shell always wins, so an invocation keeps full control of its environment.
//
// The parse reuses parseDotEnv, the same parser behind `sdt env`. A missing
// .env is a no-op; a malformed line is skipped by the parser (it never aborts
// the command). The global --no-env flag disables the whole step.
func initProjectEnv(cmd *cobra.Command) {
	if getPersistentBool(cmd, "no-env") {
		return
	}

	path, ok := findDotEnvPath()
	if !ok {
		return
	}

	entries, err := readDotEnvFile(path)
	if err != nil {
		slog.Debug("project env skipped", "path", path, "err", err)
		return
	}

	loaded := 0
	for _, e := range entries {
		if _, present := os.LookupEnv(e.Key); present {
			continue
		}
		if err := os.Setenv(e.Key, e.Value); err != nil {
			slog.Debug("project env var skipped", "key", e.Key, "err", err)
			continue
		}
		loaded++
	}

	if loaded > 0 {
		slog.Debug("project env loaded", "path", path, "vars", loaded)
	}
}

// findDotEnvPath walks up from the current working directory looking for a
// .env file, mirroring findProjectConfig for .sdt.yaml. It returns the file
// path and true when one is found, or an empty path and false otherwise.
func findDotEnvPath() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		path := filepath.Join(dir, ".env")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
