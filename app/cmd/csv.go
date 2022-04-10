package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var csvJsonCmd = &cobra.Command{
	Use:   "csv2json",
	Short: "CSV to JSON",
	Long:  `Convert CSV to JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		obj, err := cmd.Flags().GetBool("object")
		exitWithError(err)

		r := csv.NewReader(strings.NewReader(str))

		if obj {

			res := make([]map[string]string, 0)

			head, err := r.Read()
			exitWithError(err)

			size := len(head)

			for {
				record, err := r.Read()

				if err != nil {
					if err == io.EOF {
						break
					}
					exitWithError(err)
				}

				row := make(map[string]string)
				for i := 0; i < size; i++ {
					row[head[i]] = record[i]
				}

				res = append(res, row)
			}

			js, err := json.Marshal(res)
			exitWithError(err)

			fmt.Print(string(js))

		} else {

			res := make([][]string, 0)

			for {
				record, err := r.Read()

				if err != nil {
					if err == io.EOF {
						break
					}
					exitWithError(err)
				}

				res = append(res, record)
			}

			js, err := json.Marshal(res)
			exitWithError(err)

			fmt.Print(string(js))
		}
	},
}

func init() {
	csvJsonCmd.PersistentFlags().BoolP("object", "o", false, "Rows as objects")

	rootCmd.AddCommand(csvJsonCmd)
}
