package cmd

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	fmtJSON    = "json"
	fmtYAML    = "yaml"
	fmtTOML    = "toml"
	fmtMsgpack = "msgpack"
	fmtCSV     = "csv"
)

func parseCsv(cmd *cobra.Command, str string) interface{} {
	obj := getBoolFlag(cmd, "object", false)
	sep := getStringFlag(cmd, "separator", false)

	r := csv.NewReader(strings.NewReader(str))
	r.Comma = rune(sep[0])

	if obj {
		res := make([]map[string]string, 0)
		head, err := r.Read()
		exitWithError(cmd, err)
		size := len(head)

		for {
			record, err := r.Read()

			if err != nil {
				if err == io.EOF {
					break
				}
				exitWithError(cmd, err)
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
			exitWithError(cmd, err)
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
	exitWithError(cmd, w.WriteAll(arr))
	exitWithError(cmd, w.Error())
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
			exitWithError(cmd, fmt.Errorf(`input and output formats whould be different`))
		}

		var data any
		var out []byte

		switch from {
		default:
			exitWithError(cmd, fmt.Errorf(`invalid "in" flag value "%v"`, from))
		case fmtJSON:
			exitWithError(cmd, json.Unmarshal(in, &data))
		case fmtYAML:
			exitWithError(cmd, yaml.Unmarshal(in, &data))
		case fmtTOML:
			exitWithError(cmd, toml.Unmarshal(in, &data))
		case fmtMsgpack:
			exitWithError(cmd, msgpack.Unmarshal(in, &data))
		case fmtCSV:
			data = parseCsv(cmd, string(in))
		}

		var err error

		switch to {
		default:
			err = fmt.Errorf(`invalid "out" flag value "%v"`, to)
		case fmtJSON:
			out, err = json.Marshal(&data)
		case fmtYAML:
			out, err = yaml.Marshal(&data)
		case fmtTOML:
			out, err = toml.Marshal(&data)
		case fmtMsgpack:
			out, err = msgpack.Marshal(data)
		case fmtCSV:
			out, err = buildCsv(cmd, data)
		}
		exitWithError(cmd, err)

		outputBytes(cmd, out)
	},
}

func init() {
	pf := convCmd.PersistentFlags()
	pf.StringP("in", "a", "", "Input format (json, yaml, toml, csv, msgpack)")
	pf.StringP("out", "b", "", "Output format (json, yaml, toml, csv, msgpack)")
	pf.BoolP("object", "o", false, "CSV rows as objects")
	pf.StringP("separator", "s", ",", "CSV separator")

	rootCmd.AddCommand(convCmd)
}
