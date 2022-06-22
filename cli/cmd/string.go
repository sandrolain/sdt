package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// From https://stackoverflow.com/a/56129336
func substr(input string, start int, end int) string {
	asRunes := []rune(input)

	var length int

	if start < 0 {
		start = len(asRunes) + start
	}

	if end < 0 {
		end = len(asRunes) + end
	}

	if end < start {
		end = start
	}

	length = end - start

	if start >= len(asRunes) {
		return ""
	}

	if start+length > len(asRunes) {
		length = len(asRunes) - start
	}

	return string(asRunes[start : start+length])
}

func splitStringParts(cmd *cobra.Command, str string) (string, string, string) {
	start := getIntFlag(cmd, "start", false)
	end := getIntFlag(cmd, "end", false)
	var a, b, c string
	a = substr(str, 0, start)
	b = substr(str, start, end)
	if end == 0 {
		end = len(str)
	}
	c = substr(str, end, len(str))
	return a, b, c
}

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
		a, b, c := splitStringParts(cmd, str)
		out := a + strings.ToUpper(b) + c
		outputString(cmd, out)
	},
}

var lowerCaseCmd = &cobra.Command{
	Use:     "lowercase",
	Aliases: []string{"lc"},
	Short:   "Lowercase string",
	Long:    `Lowercase string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		a, b, c := splitStringParts(cmd, str)
		out := a + strings.ToLower(b) + c
		outputString(cmd, out)
	},
}

var titleCaseCmd = &cobra.Command{
	Use:     "titlecase",
	Aliases: []string{"tc", "capital case", "cc"},
	Short:   "Title Case string",
	Long:    `Title Case string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		a, b, c := splitStringParts(cmd, str)
		out := a + cases.Title(language.Und).String(b) + c
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
		a, b, c := splitStringParts(cmd, str)
		byt := must(json.Marshal(b))
		out := a + string(byt[1:len(byt)-1]) + c
		outputString(cmd, out)
	},
}

var unescapeCmd = &cobra.Command{
	Use:     "unescape",
	Aliases: []string{"uesc"},
	Short:   "Unescape string",
	Long:    `Unescape string`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		a, b, c := splitStringParts(cmd, str)
		b = fmt.Sprintf(`"%s"`, b)
		var res string
		exitWithError(json.Unmarshal([]byte(b), &res))
		out := a + res + c
		outputString(cmd, out)
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
		a, b, c := splitStringParts(cmd, str)
		out := a + replaceSpaces(b, sub) + c
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
	pf := stringCmd.PersistentFlags()
	pf.IntP("start", "s", 0, "Start index")
	pf.IntP("end", "e", math.MaxInt, "End index")

	stringCmd.AddCommand(upperCaseCmd)
	stringCmd.AddCommand(lowerCaseCmd)
	stringCmd.AddCommand(titleCaseCmd)
	stringCmd.AddCommand(escapeCmd)
	stringCmd.AddCommand(unescapeCmd)
	stringCmd.AddCommand(countCmd)

	pf = replaceSpaceCmd.PersistentFlags()
	pf.StringP("replace", "r", "", "String for replace")
	stringCmd.AddCommand(replaceSpaceCmd)

	rootCmd.AddCommand(stringCmd)
}
