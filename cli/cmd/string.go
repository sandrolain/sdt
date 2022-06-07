package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var stringCmd = &cobra.Command{
	Use:     "string",
	Aliases: []string{"str"},
	Short:   "String Tools",
	Long:    `String Tools`,
}

var upperCaseCmd = &cobra.Command{
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

var lowerCaseCmd = &cobra.Command{
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

var titleCaseCmd = &cobra.Command{
	Use:     "titlecase",
	Aliases: []string{"tc"},
	Short:   "Titlecase string",
	Long:    `Titlecase string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		c := cases.Title(language.Und)
		out := c.String(str)
		outputString(cmd, out)
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

func replaceSpaces(str string, sub string) string {
	return must(regexp.Compile(`\s+`)).ReplaceAllString(str, sub)
}

var replaceSpaceCmd = &cobra.Command{
	Use:     "replacespace",
	Aliases: []string{"rsp"},
	Short:   "Replace Spaces",
	Long:    `Replace Spaces`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		sub := getStringFlag(cmd, "replace", false)
		out := replaceSpaces(str, sub)
		outputString(cmd, out)
	},
}

var countCmd = &cobra.Command{
	Use:     "count",
	Aliases: []string{"cnt"},
	Short:   "Count text elements",
	Long:    `Count text elements`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)

		lines := len(strings.Split(str, "\n"))
		words := len(strings.Fields(str))
		chars := len(str)

		res := map[string]int{
			"lines":      lines,
			"words":      words,
			"characters": chars,
		}
		out := must(json.Marshal(res))

		outputBytes(cmd, out)
	},
}

func init() {
	stringCmd.AddCommand(upperCaseCmd)
	stringCmd.AddCommand(lowerCaseCmd)
	stringCmd.AddCommand(titleCaseCmd)
	stringCmd.AddCommand(escapeCmd)
	stringCmd.AddCommand(unescapeCmd)
	stringCmd.AddCommand(countCmd)

	pf := replaceSpaceCmd.PersistentFlags()
	pf.StringP("replace", "r", "", "String for replace")
	stringCmd.AddCommand(replaceSpaceCmd)

	rootCmd.AddCommand(stringCmd)
}
