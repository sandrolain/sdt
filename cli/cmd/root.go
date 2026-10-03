package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/sandrolain/sdt/cli/utils"
	"github.com/sandrolain/sdt/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// getFormat returns the value of the global --format flag (default "text").
func getFormat(cmd *cobra.Command) string {
	if cmd != nil {
		if f := cmd.Root().PersistentFlags().Lookup("format"); f != nil {
			v, err := cmd.Root().PersistentFlags().GetString("format")
			if err == nil {
				return v
			}
		}
	}
	return "text"
}

// getPersistentBool returns the value of a root persistent bool flag, or false
// when the flag is not defined on the root command.
func getPersistentBool(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	pf := cmd.Root().PersistentFlags()
	if f := pf.Lookup(name); f != nil {
		if v, err := pf.GetBool(name); err == nil {
			return v
		}
	}
	return false
}

// getPersistentString returns the value of a root persistent string flag, or
// def when the flag is not defined on the root command.
func getPersistentString(cmd *cobra.Command, name, def string) string {
	if cmd == nil {
		return def
	}
	pf := cmd.Root().PersistentFlags()
	if f := pf.Lookup(name); f != nil {
		if v, err := pf.GetString(name); err == nil {
			return v
		}
	}
	return def
}

const logo = `
                             dddddddd
                             d::::::d        tttt
                             d::::::d     ttt:::t
                             d::::::d     t:::::t
                             d:::::d      t:::::t
    ssssssssss       ddddddddd:::::dttttttt:::::ttttttt
  ss::::::::::s    dd::::::::::::::dt:::::::::::::::::t
ss:::::::::::::s  d::::::::::::::::dt:::::::::::::::::t
s::::::ssss:::::sd:::::::ddddd:::::dtttttt:::::::tttttt
 s:::::s  ssssss d::::::d    d:::::d      t:::::t
   s::::::s      d:::::d     d:::::d      t:::::t
      s::::::s   d:::::d     d:::::d      t:::::t
ssssss   s:::::s d:::::d     d:::::d      t:::::t    tttttt
s:::::ssss::::::sd::::::ddddd::::::dd     t::::::tttt:::::t
s::::::::::::::s  d:::::::::::::::::d     tt::::::::::::::t
 s:::::::::::ss    d:::::::::ddd::::d       tt:::::::::::tt
  sssssssssss       ddddddddd   ddddd         ttttttttttt

`

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sdt",
	Short: "Smart Developer Tools",
	Long:  logo + `Smart Developer Tools is a collection of CLI utilities for developers`,
	// installLogger runs after flag parsing and before the command body, so
	// every command logs through the same handler. initProjectEnv then loads a
	// project-root .env so commands can read project credentials (never
	// overriding a value already present in the shell environment).
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		installLogger(cmd)
		initProjectEnv(cmd)
		return nil
	},
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.String("input", "", "Input String")
	pf.BytesBase64("inb64", []byte{}, "Input Base 64")
	pf.String("file", "", "Input File")
	pf.String("format", "text", "Output format: text|json|yaml")
	pf.Bool("quiet", false, "Suppress informational messages, only output result")
	pf.Bool("no-color", false, "Disable ANSI color codes")
	pf.Bool("no-env", false, "Do not load a project-root .env file")
	pf.String("log-format", "text", "Log format: text|json")
	// Install a plain logger up front so errors raised before flag parsing
	// (an unknown flag, for instance) render like every other log line.
	installLogger(rootCmd)
}

// installLogger wires the shared logger to the command error stream, keeping
// stdout as the result channel. Colour is decided by internal/logging: only on
// a terminal, and never with --no-color or NO_COLOR set. --quiet raises the
// level to Warn.
func installLogger(cmd *cobra.Command) {
	format := getPersistentString(cmd, "log-format", "text")
	switch format {
	case "text", "json":
	default:
		exitWithError(cmd, fmt.Errorf("invalid --log-format %q: use text or json", format))
		return
	}

	level := slog.LevelInfo
	if getPersistentBool(cmd, "quiet") {
		level = slog.LevelWarn
	}

	logging.Setup(logging.Options{
		Writer:  cmd.ErrOrStderr(),
		Level:   level,
		NoColor: getPersistentBool(cmd, "no-color"),
		JSON:    format == "json",
		Time:    false,
	})
}

func Execute() {
	loadFileConfig()
	exitWithError(nil, rootCmd.Execute())
}

func getInputStringOrFlag(cmd *cobra.Command, args []string, flag string, required bool) string {
	val := getInputString(cmd, args)
	if len(val) == 0 {
		val = getStringFlag(cmd, flag, required)
	}
	return val
}

// func getInputStringRequired(cmd *cobra.Command, args []string) string {
// 	res := getInputString(cmd, args)
// 	if len(res) == 0 {
// 		exitWithError(cmd, fmt.Errorf("primary command input should not be empty"))
// 	}
// 	return res
// }

func getInputBytesRequired(cmd *cobra.Command, args []string) []byte {
	res := getInputBytes(cmd, args)
	if len(res) == 0 {
		exitWithError(cmd, fmt.Errorf("primary command input should not be empty"))
	}
	return res
}

var exit func(code int) = os.Exit

// exitWithError reports err through the shared logger and exits with code 1.
// The logger writes to the command error stream, so stdout keeps carrying
// results only. cmd is accepted so call sites stay uniform; the installed
// logger already knows where to write.
func exitWithError(_ *cobra.Command, err error) {
	if err == nil {
		return
	}
	slog.Error(err.Error())
	exit(1)
}

func getFlag[T any](cmd *cobra.Command, name string, required bool, fFlags func(flags *pflag.FlagSet) (T, error), fFile func() T) T {
	var val T
	var err error
	found := false
	flags := cmd.Flags()
	if flags.Changed(name) {
		val, err = fFlags(flags)
		exitWithError(cmd, err)
		found = true
	} else {
		key := getUsePath(cmd, name)
		if viper.IsSet(key) {
			val = fFile()
			found = true
		}
	}

	if !found {
		if required {
			exitWithError(cmd, fmt.Errorf("the flag \"%s\" is required", name))
		} else {
			val, err = fFlags(flags)
			exitWithError(cmd, err)
		}
	}
	return val
}

func getIntFlag(cmd *cobra.Command, name string, required bool) int {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) (int, error) {
		return flags.GetInt(name)
	}, func() int {
		return viper.GetInt(name)
	})
}

func getInt64Flag(cmd *cobra.Command, name string, required bool) int64 {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) (int64, error) {
		return flags.GetInt64(name)
	}, func() int64 {
		return viper.GetInt64(name)
	})
}

func getUintFlag(cmd *cobra.Command, name string, required bool) uint {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) (uint, error) {
		return flags.GetUint(name)
	}, func() uint {
		return viper.GetUint(name)
	})
}

func getBoolFlag(cmd *cobra.Command, name string, required bool) bool {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) (bool, error) {
		return flags.GetBool(name)
	}, func() bool {
		return viper.GetBool(name)
	})
}

func getStringFlag(cmd *cobra.Command, name string, required bool) string {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) (string, error) {
		return flags.GetString(name)
	}, func() string {
		return viper.GetString(name)
	})
}

func getBytesBase64Flag(cmd *cobra.Command, name string, required bool) []byte {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) ([]byte, error) {
		return flags.GetBytesBase64(name)
	}, func() []byte {
		byt, err := utils.Base64Decode(viper.GetString(name))
		exitWithError(cmd, err)
		return byt
	})
}

func getStringArrayFlag(cmd *cobra.Command, name string, required bool) []string {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) ([]string, error) {
		return flags.GetStringArray(name)
	}, func() []string {
		return viper.GetStringSlice(name)
	})
}

func getUsePath(cmd *cobra.Command, name string) string {
	uses := getUseArray(cmd)
	uses = append(uses, name)
	return strings.Join(uses, ".")
}

func getUseArray(cmd *cobra.Command) []string {
	uses := []string{}
	for {
		uses = append([]string{cmd.Name()}, uses...)
		cmd = cmd.Parent()
		if cmd == nil || cmd.Use == "sdt" {
			break
		}
	}
	return uses
}

func outputBytes(cmd *cobra.Command, byt []byte) {
	_, e := cmd.OutOrStdout().Write(byt)
	exitWithError(cmd, e)
}

func outputString(cmd *cobra.Command, str string) {
	_, e := cmd.OutOrStdout().Write([]byte(str))
	exitWithError(cmd, e)
}
