package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docCmd = &cobra.Command{
	Use:   "docs",
	Short: "Generate CLI docs",
	Long:  `Generate CLI documentation`,
	Run: func(cmd *cobra.Command, args []string) {
		out, err := cmd.Flags().GetString("out")
		exitWithError(err)

		os.MkdirAll(out, os.ModePerm)

		err = doc.GenMarkdownTree(rootCmd, out)
		exitWithError(err)
	},
}

func init() {
	docCmd.PersistentFlags().StringP("out", "o", "./docs/", "Output directory")

	rootCmd.AddCommand(docCmd)
}
