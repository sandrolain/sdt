//go:build !wasm

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ProjectConfig holds the project identity and the search scope declared in
// .sdt.yaml.
type ProjectConfig struct {
	Project string `json:"project" yaml:"project"`
	Group   string `json:"group"   yaml:"group"`
	// SkipDirs is the project's declared content-verb scope: glob patterns,
	// relative to the project root, pruned from the working tree. Declaring it
	// replaces the shipped defaults rather than extending them; leaving it unset
	// uses them, which is why the defaults ship as comments below and not as a
	// value here.
	SkipDirs []string `json:"skipDirs,omitempty" yaml:"skipDirs,omitempty"`
}

// skipDirsComment is the create-only note written under a new .sdt.yaml. It
// documents the key, its shape and the shipped defaults **without** taking a
// value: a written default would stop being a default, freezing every
// bootstrapped project to whatever shipped on the day it was created.
const skipDirsComment = `# skipDirs — globs pruned from the working tree, relative to this file.
# Declaring a list REPLACES the defaults below, it does not extend them.
# Unset means: .git, .hg, .svn, node_modules, .venv
# skipDirs:
#   - dist
#   - .next`

// buildProjectConfigContent renders the YAML body of a .sdt.yaml file. A
// declared skipDirs is rendered as real values; otherwise the create-only
// comment documents the mechanism and leaves the key absent, so the product stays
// free to change its defaults.
func buildProjectConfigContent(project, group string, skipDirs []string) string {
	lines := []string{fmt.Sprintf("project: %s", project)}
	if group != "" {
		lines = append(lines, fmt.Sprintf("group: %s", group))
	}
	if len(skipDirs) > 0 {
		lines = append(lines, "skipDirs:")
		for _, d := range skipDirs {
			lines = append(lines, fmt.Sprintf("  - %s", d))
		}
	} else {
		lines = append(lines, skipDirsComment)
	}
	return strings.Join(lines, "\n") + "\n"
}

// findProjectConfig walks up from the current working directory looking for
// a .sdt.yaml file. Returns nil, nil when no file is found.
func findProjectConfig() (*ProjectConfig, error) {
	cfg, _, err := findProjectConfigAt()
	return cfg, err
}

// findProjectConfigAt walks up like findProjectConfig and also reports the
// directory the file was found in. The directory is what a project-root-relative
// pattern resolves against, so a caller that needs one cannot use the config
// alone. Returns nil, "", nil when no file is found; a malformed file degrades to
// nil rather than failing, as every other config read in this codebase does.
func findProjectConfigAt() (*ProjectConfig, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	for {
		path := filepath.Join(dir, ".sdt.yaml")
		data, err := os.ReadFile(path) //#nosec G304 -- user-controlled project root
		if err == nil {
			var cfg ProjectConfig
			if uerr := yaml.Unmarshal(data, &cfg); uerr == nil {
				return &cfg, dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, "", nil
}

// getProjectAndGroup resolves project and group from (in priority order):
//  1. --project / --group explicit flags
//  2. .sdt.yaml found walking up from CWD
//
// Returns empty strings when neither source provides a value.
func getProjectAndGroup(cmd *cobra.Command) (project, group string) {
	if f := cmd.Flags().Lookup("project"); f != nil && f.Changed {
		if v, err := cmd.Flags().GetString("project"); err == nil {
			project = v
		}
	}
	if f := cmd.Flags().Lookup("group"); f != nil && f.Changed {
		if v, err := cmd.Flags().GetString("group"); err == nil {
			group = v
		}
	}
	if project == "" || group == "" {
		cfg, err := findProjectConfig()
		if err != nil {
			cfg = nil
		}
		if cfg != nil {
			if project == "" {
				project = cfg.Project
			}
			if group == "" {
				group = cfg.Group
			}
		}
	}
	return
}

func loadFileConfig() {
	viper.SetConfigName(sdtConfigFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			exitWithError(nil, err)
		}
	}
}

func getInputString(cmd *cobra.Command, args []string) string {
	flags := cmd.Flags()

	if flags.Lookup("file").Changed {
		file := getStringFlag(cmd, "file", false)

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}

		//#nosec G304 -- implementation of generic utility
		res, err := os.ReadFile(file)
		exitWithError(cmd, err)
		return string(res)
	}

	if flags.Lookup("input").Changed {
		return getStringFlag(cmd, "input", true)
	}

	if flags.Lookup("inb64").Changed {
		return string(getBytesBase64Flag(cmd, "inb64", true))
	}

	if len(args) > 0 {
		return strings.Join(args[:], "")
	}
	res, err := io.ReadAll(cmd.InOrStdin())
	exitWithError(cmd, err)
	return string(res)
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	flags := cmd.Flags()

	if flags.Lookup("file").Changed {
		file := getStringFlag(cmd, "file", false)

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}
		//#nosec G304 -- implementation of generic utility
		res, err := os.ReadFile(file)
		exitWithError(cmd, err)
		return res
	}

	if flags.Lookup("input").Changed {
		return []byte(getStringFlag(cmd, "input", true))
	}

	if flags.Lookup("inb64").Changed {
		return getBytesBase64Flag(cmd, "inb64", true)
	}

	if len(args) > 0 {
		return []byte(strings.Join(args[:], ""))
	}

	res, err := io.ReadAll(cmd.InOrStdin())
	exitWithError(cmd, err)
	return res
}

func ExecuteByArgs(args []string, in []byte) ([]byte, error) {
	inr := bytes.NewReader(in)
	rootCmd.SetIn(inr)

	origOut := rootCmd.OutOrStdout()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	rootCmd.SetIn(nil)
	rootCmd.SetOut(origOut)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
