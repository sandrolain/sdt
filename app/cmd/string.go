package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var uppercaseCmd = &cobra.Command{
	Use:     "uppercase",
	Aliases: []string{"uc"},
	Short:   "Uppercase string",
	Long:    `Uppercase string`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(cmd, args)
		exitWithError(err)

		res := strings.ToUpper(str)
		fmt.Print(res)
	},
}

var lowercaseCmd = &cobra.Command{
	Use:     "lowercase",
	Aliases: []string{"lc"},
	Short:   "Lowercase string",
	Long:    `Lowercase string`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(cmd, args)
		exitWithError(err)

		res := strings.ToLower(str)
		fmt.Print(res)
	},
}

var escapeCmd = &cobra.Command{
	Use:     "escape",
	Aliases: []string{"esc"},
	Short:   "Escape string",
	Long:    `Escape string`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(cmd, args)
		exitWithError(err)

		j, err := json.Marshal(str)
		exitWithError(err)

		str = string(j[1 : len(j)-1])
		fmt.Print(str)
	},
}

var unescapeCmd = &cobra.Command{
	Use:     "unescape",
	Aliases: []string{"uesc"},
	Short:   "Unescape string",
	Long:    `Unescape string`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(cmd, args)
		exitWithError(err)

		str = fmt.Sprintf(`"%s"`, str)

		var res string
		err = json.Unmarshal([]byte(str), &res)
		exitWithError(err)

		fmt.Print(res)
	},
}

func init() {
	rootCmd.AddCommand(uppercaseCmd)
	rootCmd.AddCommand(lowercaseCmd)
	rootCmd.AddCommand(escapeCmd)
	rootCmd.AddCommand(unescapeCmd)
}
