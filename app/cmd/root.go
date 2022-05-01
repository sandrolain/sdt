package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/sandrolain/sdt/app/utils"
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
	viper.SetConfigName("sdt")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("err: %v\n", err)
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			exitWithError(err)
		}
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getInputString(cmd *cobra.Command, args []string) string {
	file, err := cmd.InheritedFlags().GetString("file")
	exitWithError(err)

	if file != "" {
		exist, err := fileExists(file)
		exitWithError(err)
		if !exist {
			exitWithError(fmt.Errorf(`file "%s" not exist`, file))
		}

		content, err := ioutil.ReadFile(file)
		exitWithError(err)

		return string(content)
	}

	input, err := cmd.InheritedFlags().GetBool("input")
	exitWithError(err)

	if input {
		byt, err := ioutil.ReadAll(os.Stdin)
		exitWithError(err)
		if len(byt) > 0 {
			return string(byt)
		}
	}

	fi, err := os.Stdin.Stat()
	exitWithError(err)

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return ""
		}
		if len(byt) > 0 {
			return string(byt)
		}
	}

	if len(args) > 0 {
		return args[0]
	}

	return ""
}

func getInputStringOrFlag(cmd *cobra.Command, args []string, flag string, required bool) string {
	val := getInputString(cmd, args)
	if len(val) == 0 {
		val = getStringFlag(cmd, flag, required)
	}
	return val
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	file, err := cmd.InheritedFlags().GetString("file")
	exitWithError(err)

	if file != "" {
		exist, err := fileExists(file)
		exitWithError(err)
		if !exist {
			exitWithError(fmt.Errorf(`file "%s" not exist`, file))
		}

		content, err := ioutil.ReadFile(file)
		exitWithError(err)

		return content
	}

	input, err := cmd.InheritedFlags().GetBool("input")
	exitWithError(err)

	if input {
		res, err := ioutil.ReadAll(os.Stdin)
		exitWithError(err)
		return res
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		exitWithError(err)

		if len(byt) > 0 {
			return byt
		}
	}

	if len(args) > 0 {
		return []byte(args[0])
	}

	return []byte{}
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
		log.Fatal(err)
	}
}

func exitWithErrorF(f string, err error) {
	if err != nil {
		log.Fatalf(f, err)
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
