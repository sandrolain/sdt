package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// from: https://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-exists
func fileExists(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

var fileReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read File",
	Long:  `Read File`,
	Run: func(cmd *cobra.Command, args []string) {
		file := getStringFlag(cmd, "file", true)

		exist, err := fileExists(file)
		exitWithError(err)
		if !exist {
			exitWithError(fmt.Errorf(`file "%s" not exist`, file))
		}

		content, err := ioutil.ReadFile(file)
		exitWithError(err)

		fmt.Print(string(content))
	},
}

var fileWriteCmd = &cobra.Command{
	Use:   "write",
	Short: "File Write",
	Long:  `File Write`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)

		file := getStringFlag(cmd, "file", true)
		overwrite := getBoolFlag(cmd, "overwrite", false)

		file, err := filepath.Abs(file)
		exitWithError(err)

		exist, err := fileExists(file)
		exitWithError(err)
		if exist && !overwrite {
			exitWithError(fmt.Errorf(`file "%s" already exist`, file))
		}

		err = ioutil.WriteFile(file, byt, 0666)
		exitWithError(err)

		fmt.Print(file)
	},
}

func init() {
	fileReadCmd.PersistentFlags().StringP("file", "p", "", "File path")
	fileWriteCmd.PersistentFlags().StringP("file", "p", "", "File path")
	fileWriteCmd.PersistentFlags().BoolP("overwrite", "o", false, "Overwrite if already exist")

	rootCmd.AddCommand(fileReadCmd)
	rootCmd.AddCommand(fileWriteCmd)
}
