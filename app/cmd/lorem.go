package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	lorem "github.com/drhodes/golorem"
)

var loremCmd = &cobra.Command{
	Use:   "lorem",
	Short: "Lorem Ipsum",
	Long:  `Generate Lorem Ipsum`,
	Run: func(cmd *cobra.Command, args []string) {
		typ, err := cmd.Flags().GetString("type")
		exitWithError(err)
		min, err := cmd.Flags().GetInt("min")
		exitWithError(err)
		max, err := cmd.Flags().GetInt("max")
		exitWithError(err)

		var res string

		switch typ {
		case "word":
			res = lorem.Word(min, max)
			break
		default:
		case "sentence":
			res = lorem.Sentence(min, max)
			break
		case "paragraph":
			res = lorem.Paragraph(min, max)
			break
		}

		fmt.Print(res)
	},
}

func init() {
	loremCmd.PersistentFlags().StringP("type", "t", "paragraph", "Image size")
	loremCmd.PersistentFlags().IntP("min", "n", 1, "Min")
	loremCmd.PersistentFlags().IntP("max", "m", 10, "Max")

	rootCmd.AddCommand(loremCmd)
}
