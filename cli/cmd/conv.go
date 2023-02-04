package cmd

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/hetiansu5/urlquery"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func parseCsv(cmd *cobra.Command, str string) interface{} {
	obj := getBoolFlag(cmd, "object", false)
	sep := getStringFlag(cmd, "separator", false)

	r := csv.NewReader(strings.NewReader(str))
	r.Comma = rune(sep[0])

	if obj {
		res := make([]map[string]string, 0)
		head := must(r.Read())
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

func buildCsv(cmd *cobra.Command, data any) ([]byte, error) {
	sep := getStringFlag(cmd, "separator", false)

	badDataErr := "input data must be an array of strings' arrays for conversion to CSV (%v)"

	dataArr, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf(badDataErr, 1)
	}
	arr := make([][]string, len(dataArr))
	rowLen := -1
	for i, val := range dataArr {
		valArr, ok := val.([]interface{})
		if !ok {
			return nil, fmt.Errorf(badDataErr, 2)
		}
		actLen := len(valArr)
		if rowLen < 0 {
			rowLen = actLen
		}

		if actLen != rowLen {
			return nil, fmt.Errorf("all rows must have same items number for conversion to CSV (row %v is %v != %v)", i, actLen, rowLen)
		}

		arr[i] = make([]string, actLen)
		for j, str := range valArr {
			v, ok := str.(string)
			if !ok {
				return nil, fmt.Errorf(badDataErr, 3)
			}
			arr[i][j] = v
		}
	}

	b := new(bytes.Buffer)
	w := csv.NewWriter(b)
	w.Comma = rune(sep[0])
	exitWithError(w.WriteAll(arr))
	exitWithError(w.Error())
	return b.Bytes(), nil
}

var convCmd = &cobra.Command{
	Use:   "conv",
	Short: "Data Conversion",
	Long:  `Data Conversion`,
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
			exitWithError(fmt.Errorf(`invalid "in" flag value "%v"`, from))
		case "json":
			exitWithError(json.Unmarshal(in, &data))
		case "yaml":
			exitWithError(yaml.Unmarshal(in, &data))
		case "toml":
			exitWithError(toml.Unmarshal(in, &data))
		case "query":
			exitWithError(urlquery.Unmarshal(in, &data))
		case "csv":
			data = parseCsv(cmd, string(in))
		}

		switch to {
		default:
			exitWithError(fmt.Errorf(`invalid "out" flag value "%v"`, to))
		case "json":
			out = must(json.Marshal(&data))
		case "yaml":
			out = must(yaml.Marshal(&data))
		case "toml":
			out = must(toml.Marshal(&data))
		case "query":
			out = must(urlquery.Marshal(data))
		case "csv":
			out = must(buildCsv(cmd, data))
		}

		outputBytes(cmd, out)
	},
}

func init() {
	pf := convCmd.PersistentFlags()
	pf.StringP("in", "a", "", "Input format (json, yaml, toml, query, csv)")
	pf.StringP("out", "b", "", "Output format (json, yaml, toml, query, csv)")
	pf.BoolP("object", "o", false, "CSV rows as objects")
	pf.StringP("separator", "s", ",", "CSV separator")

	rootCmd.AddCommand(convCmd)
}
