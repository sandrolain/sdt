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
		str := getInputString(cmd, args)
		res := strings.ToUpper(str)
		outputString(cmd, res)
	},
}

var lowercaseCmd = &cobra.Command{
	Use:     "lowercase",
	Aliases: []string{"lc"},
	Short:   "Lowercase string",
	Long:    `Lowercase string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		res := strings.ToLower(str)
		outputString(cmd, res)
	},
}

var escapeCmd = &cobra.Command{
	Use:     "escape",
	Aliases: []string{"esc"},
	Short:   "Escape string",
	Long:    `Escape string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		j := must(json.Marshal(str))
		str = string(j[1 : len(j)-1])
		outputString(cmd, str)
	},
}

var unescapeCmd = &cobra.Command{
	Use:     "unescape",
	Aliases: []string{"uesc"},
	Short:   "Unescape string",
	Long:    `Unescape string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		str = fmt.Sprintf(`"%s"`, str)
		var res string
		exitWithError(json.Unmarshal([]byte(str), &res))
		outputString(cmd, res)
	},
}

func init() {
	rootCmd.AddCommand(uppercaseCmd)
	rootCmd.AddCommand(lowercaseCmd)
	rootCmd.AddCommand(escapeCmd)
	rootCmd.AddCommand(unescapeCmd)
}
