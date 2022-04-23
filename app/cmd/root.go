package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
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

func getInputString(cmd *cobra.Command, args []string) (string, error) {
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

		return string(content), nil
	}

	input, err := cmd.InheritedFlags().GetBool("input")
	exitWithError(err)

	if input {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		if len(byt) > 0 {
			return string(byt), nil
		}
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		if len(byt) > 0 {
			return string(byt), nil
		}
	}

	if len(args) > 0 {
		return args[0], nil
	}

	return "", nil
}

func getInputBytes(cmd *cobra.Command, args []string) ([]byte, error) {
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

		return content, nil
	}

	input, err := cmd.InheritedFlags().GetBool("input")
	exitWithError(err)

	if input {
		return ioutil.ReadAll(os.Stdin)
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return byt, err
		}
		if len(byt) > 0 {
			return byt, nil
		}
	}

	if len(args) > 0 {
		return []byte(args[0]), nil
	}

	return []byte{}, nil
}

func getInputBytesRequired(cmd *cobra.Command, args []string) ([]byte, error) {
	res, err := getInputBytes(cmd, args)
	if err != nil {
		return res, err
	}
	if len(res) == 0 {
		return res, fmt.Errorf("primary command input should not be empty")
	}
	return res, err
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

func getIntFlag(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	exitWithError(err)
	return val
}

func getUintFlag(cmd *cobra.Command, name string) uint {
	val, err := cmd.Flags().GetUint(name)
	exitWithError(err)
	return val
}

func getBoolFlag(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	exitWithError(err)
	return val
}

func getStringFlag(cmd *cobra.Command, name string) string {
	var val string
	var err error
	if cmd.Flags().Changed(name) {
		val, err = cmd.Flags().GetString(name)
		exitWithError(err)
	} else {
		key := getUsePath(cmd, name)
		if viper.IsSet(key) {
			val = viper.GetString(key)
		}
	}
	return val
}

func getUsePath(cmd *cobra.Command, name string) string {
	uses := []string{name}

	for true {
		uses = append([]string{cmd.Use}, uses...)
		cmd = cmd.Parent()
		if cmd == nil || cmd.Use == "sdt" {
			break
		}
	}

	return strings.Join(uses, ".")
}

func getStringArrayFlag(cmd *cobra.Command, name string) []string {
	val, err := cmd.Flags().GetStringArray(name)
	exitWithError(err)
	return val
}
