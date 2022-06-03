package cmd

import (
	"github.com/spf13/cobra"

	lorem "github.com/drhodes/golorem"
)

var loremCmd = &cobra.Command{
	Use:   "lorem",
	Short: "Lorem Ipsum",
	Long:  `Generate Lorem Ipsum`,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", false)
		min := getIntFlag(cmd, "min", false)
		max := getIntFlag(cmd, "max", false)

		var res string

		switch typ {
		case "sentence":
			res = lorem.Sentence(min, max)
		case "word":
			res = lorem.Word(min, max)
		case "paragraph":
			res = lorem.Paragraph(min, max)
		}

		outputString(cmd, res)
	},
}

func init() {
	pf := loremCmd.PersistentFlags()
	pf.StringP("type", "t", "sentence", "Sequence type (sentence, word, paragraph)")
	pf.IntP("min", "n", 1, "Min")
	pf.IntP("max", "m", 10, "Max")

	rootCmd.AddCommand(loremCmd)
}
