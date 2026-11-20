package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func normalizeMarkdownTree(rootPath string) (resultErr error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := root.Close(); resultErr == nil {
			resultErr = closeErr
		}
	}()

	return filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != sdtMarkdownExt {
			return nil
		}

		relativePath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		data, err := root.ReadFile(relativePath)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " \t\r")
		}
		normalized := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
		if normalized == string(data) {
			return nil
		}
		return root.WriteFile(relativePath, []byte(normalized), 0o644)
	})
}

var docCmd = &cobra.Command{
	Use:   cmdDocs,
	Short: "Generate CLI docs",
	Long:  `Generate CLI documentation`,
	Run: func(cmd *cobra.Command, args []string) {
		out := getStringFlag(cmd, "out", false)
		exitWithError(cmd, os.MkdirAll(out, 0750))
		configureCommandGroups(rootCmd)
		rootCmd.DisableAutoGenTag = true
		exitWithError(cmd, doc.GenMarkdownTree(rootCmd, out))
		exitWithError(cmd, normalizeMarkdownTree(out))
	},
}

func init() {
	docCmd.PersistentFlags().StringP("out", "o", "./docs/", "Output directory")
	rootCmd.AddCommand(docCmd)
}
