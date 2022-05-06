package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func parseCsv(cmd *cobra.Command, str string) interface{} {
	obj := getBoolFlag(cmd, "object", false)

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

		return res

	}

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
	return res
}

var convCmd = &cobra.Command{
	Use:   "conv",
	Short: "Conversion Tools",
	Long:  `Conversion Tools`,
	Run: func(cmd *cobra.Command, args []string) {
		in := getInputBytes(cmd, args)
		from := getStringFlag(cmd, "in", true)
		to := getStringFlag(cmd, "out", true)

		if from == to {
			exitWithError(fmt.Errorf(`input and output formats whould be different`))
		}

		var data any
		var out []byte

		switch from {
		default:
			exitWithError(fmt.Errorf(`invalid "from" flag value "%v"`, from))
		case "json":
			exitWithError(json.Unmarshal(in, &data))
		case "yaml":
			exitWithError(yaml.Unmarshal(in, &data))
		case "csv":
			data = parseCsv(cmd, string(in))
		}

		switch to {
		default:
			exitWithError(fmt.Errorf(`invalid "from" flag value "%v"`, from))
		case "json":
			out = must(json.Marshal(&data))
		case "yaml":
			out = must(yaml.Marshal(&data))
		}

		outputBytes(cmd, out)
	},
}

func init() {
	pf := convCmd.PersistentFlags()
	pf.StringP("in", "a", "", "Input format (json, yaml, toml, csv)")
	pf.StringP("out", "b", "", "Output format (json, yaml, toml, csv)")
	pf.BoolP("object", "o", false, "Rows as objects")

	rootCmd.AddCommand(convCmd)
}
