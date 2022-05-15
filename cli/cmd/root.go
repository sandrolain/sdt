package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/gookit/color"
	"github.com/sandrolain/sdt/cli/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sdt",
	Short: "Smart Developer Tools",
	Long:  `Smart Developer Tools is a collection of CLI utilities for developers`,
}

func init() {
	rootCmd.PersistentFlags().BoolP("input", "i", false, "Input Prompt")
	rootCmd.PersistentFlags().StringP("file", "f", "", "Input File")
}

func Execute() {
	loadFileConfig()
	exitWithError(rootCmd.Execute())
}

func getInputStringOrFlag(cmd *cobra.Command, args []string, flag string, required bool) string {
	val := getInputString(cmd, args)
	if len(val) == 0 {
		val = getStringFlag(cmd, flag, required)
	}
	return val
}

func getInputBytesRequired(cmd *cobra.Command, args []string) []byte {
	res := getInputBytes(cmd, args)
	if len(res) == 0 {
		exitWithError(fmt.Errorf("primary command input should not be empty"))
	}
	return res
}

func exitWithError(err error) {
	if err != nil {
		color.Error.Println(err)
		os.Exit(1)
	}
}

func exitWithErrorF(f string, err error) {
	if err != nil {
		color.Error.Printf(f, err)
		os.Exit(1)
	}
}

func getFlag[T any](cmd *cobra.Command, name string, required bool, fFlags func(flags *pflag.FlagSet) (T, error), fFile func() T) T {
	var val T
	var err error
	found := false
	flags := cmd.Flags()
	if flags.Changed(name) {
		val, err = fFlags(flags)
		exitWithError(err)
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
			exitWithError(fmt.Errorf("the flag \"%s\" is required", name))
		} else {
			val, err = fFlags(flags)
			exitWithError(err)
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
		exitWithError(err)
		return byt
	})
}

func getStringArrayFlag(cmd *cobra.Command, name string, required bool) []string {
	return getFlag(cmd, name, required, func(flags *pflag.FlagSet) ([]string, error) {
		return flags.GetStringSlice(name)
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
		uses = append([]string{cmd.Use}, uses...)
		cmd = cmd.Parent()
		if cmd == nil || cmd.Use == "sdt" {
			break
		}
	}
	return uses
}

func outputBytes(cmd *cobra.Command, byt []byte) {
	cmd.OutOrStdout().Write(byt)
}

func outputString(cmd *cobra.Command, str string) {
	cmd.OutOrStdout().Write([]byte(str))
}

func must[T any](val T, err error) T {
	if err != nil {
		exitWithError(err)
	}
	return val
}
