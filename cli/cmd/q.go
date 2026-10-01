package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// q exit codes: 1 = path or file not found, 2 = malformed JSON or an
// unsupported input format. Both are distinct from 0 (success).
const (
	qExitNotFound  = 1
	qExitMalformed = 2
)

var qCmd = &cobra.Command{
	Use:   "q",
	Short: "Query and patch JSON files by gjson path",
	Long: `Query and patch JSON files with a gjson path: a dotted path (a.b),
array indexes (a.0), wildcards (*, ?), queries (#(...)), projections (a.#.b)
and modifiers (@reverse, @pretty, @keys, @flatten).

JSON only: a .yaml/.yml argument is rejected with a named error. Convert YAML
first with "sdt conv --in yaml --out json" (usable on stdin).

Exit codes: 0 success · 1 path/file not found · 2 malformed JSON or an
unsupported input format. Diagnostics go to stderr, never to stdout.`,
}

var qGetCmd = &cobra.Command{
	Use:   "get <file> <path>",
	Short: "Print the value at a gjson path",
	Long: `Print the value at a gjson path. A scalar prints its text, an object or an
array prints its raw JSON, so stdout stays valid JSON for a nested result.

The document is read from <file>, or from the global --input/--inb64/--file
conventions when no file is given (use - to read stdin). --format json emits
an {path,value,exists,type} envelope; --default returns a value instead of
failing when the path is absent.

Examples:
  sdt q get context/index.json 'documents.0.title'
  sdt q get context/index.json 'documents.#(kind=="analysis")#.path'
  cat data.json | sdt q get 'items.#'`,
	Args: cobra.RangeArgs(1, 2),
	Run:  runQGet,
}

var qSetCmd = &cobra.Command{
	Use:   "set <file> <path> <value>",
	Short: "Set the value at a gjson path, preserving the rest of the file",
	Long: `Set the value at a gjson path. The value is parsed as JSON when it is valid
JSON (so 3 is a number, true a bool, {"a":1} an object), otherwise it is a
plain string. <value> may be omitted when it is passed through the global
--input/--inb64/--file conventions.

Untouched bytes, key order and number literals are preserved (sjson splice).
With a real file the edit is in place; with - or piped input the result goes
to stdout.

Examples:
  sdt q set package.json 'scripts.test' 'go test ./...'
  sdt q set data.json 'items.0.done' true
  printf '%s' '{"a":1}' | sdt q set - a.b '"x"'`,
	Args: cobra.RangeArgs(2, 3),
	Run:  runQSet,
}

var qDelCmd = &cobra.Command{
	Use:   "del <file> <path>",
	Short: "Delete the value at a gjson path, preserving the rest of the file",
	Long: `Delete the value at a gjson path. A missing path is an error (exit 1). With a
real file the edit is in place; with - or piped input the result goes to
stdout.

Examples:
  sdt q del package.json 'scripts.legacy'
  cat data.json | sdt q del - 'items.0'`,
	Args: cobra.RangeArgs(1, 2),
	Run:  runQDel,
}

func qExit(err error, code int) {
	slog.Error(err.Error())
	exit(code)
}

// qReadSource resolves the JSON document. With allowGlobalDoc the global
// --input/--inb64/--file conventions replace the positional file; otherwise
// args[0] is always the file (or - for stdin). rest is the argument list
// without the file, so the path/value stay positionally stable.
func qReadSource(cmd *cobra.Command, args []string, allowGlobalDoc bool) (data []byte, rest []string, origin string, code int, err error) {
	if allowGlobalDoc && qGlobalInputSet(cmd) {
		return getInputBytes(cmd, nil), args, "", 0, nil
	}
	if len(args) == 0 {
		return nil, nil, "", qExitMalformed, fmt.Errorf("missing input file (pass a file, - for stdin, or JSON via --input/--inb64/--file)")
	}
	origin = args[0]
	lower := strings.ToLower(origin)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return nil, nil, "", qExitMalformed, fmt.Errorf("unsupported YAML input %q: convert it first with \"sdt conv --in yaml --out json\"", origin)
	}
	if origin == "-" {
		data := getInputBytes(cmd, nil)
		if code, err := qRequireValidJSON(data); err != nil {
			return nil, nil, "", code, err
		}
		return data, args[1:], "", 0, nil
	}
	data, err = os.ReadFile(origin) //#nosec G304 -- the file is the command's positional input
	if err != nil {
		return nil, nil, "", qExitNotFound, fmt.Errorf("read %s: %w", origin, err)
	}
	return data, args[1:], origin, 0, nil
}

func qGlobalInputSet(cmd *cobra.Command) bool {
	for _, name := range []string{"file", "input", "inb64"} {
		// cmd.Flag climbs the command tree to the root persistent flags.
		if f := cmd.Flag(name); f != nil && f.Changed {
			return true
		}
	}
	return false
}

func qRequireValidJSON(data []byte) (int, error) {
	if !gjson.ValidBytes(data) {
		return qExitMalformed, fmt.Errorf("malformed JSON input")
	}
	return 0, nil
}

func runQGet(cmd *cobra.Command, args []string) {
	data, rest, _, code, err := qReadSource(cmd, args, true)
	if err != nil {
		qExit(err, code)
		return
	}
	if len(rest) != 1 {
		exitWithError(cmd, fmt.Errorf("usage: sdt q get <file> <path>"))
		return
	}
	if code, err := qRequireValidJSON(data); err != nil {
		qExit(err, code)
		return
	}
	path := rest[0]
	res := gjson.GetBytes(data, path)
	if !res.Exists() {
		if cmd.Flags().Changed("default") {
			outputString(cmd, getStringFlag(cmd, "default", false)+"\n")
			return
		}
		qExit(fmt.Errorf("path %q not found", path), qExitNotFound)
		return
	}

	switch getFormat(cmd) {
	case fmtJSON:
		env := qEnvelope{Path: path, Exists: true, Type: res.Type.String(), Value: res.Value()}
		out, marshalErr := json.MarshalIndent(env, "", "  ")
		exitWithError(cmd, marshalErr)
		outputString(cmd, string(out)+"\n")
	case fmtYAML:
		env := qEnvelope{Path: path, Exists: true, Type: res.Type.String(), Value: res.Value()}
		out, marshalErr := yaml.Marshal(env)
		exitWithError(cmd, marshalErr)
		outputBytes(cmd, out)
	default:
		switch res.Type {
		case gjson.JSON:
			outputString(cmd, res.Raw+"\n")
		case gjson.Null:
			outputString(cmd, "null\n")
		default:
			outputString(cmd, res.String()+"\n")
		}
	}
}

// qEnvelope is the --format json|yaml projection of a q get result.
type qEnvelope struct {
	Path   string `json:"path" yaml:"path"`
	Exists bool   `json:"exists" yaml:"exists"`
	Type   string `json:"type" yaml:"type"`
	Value  any    `json:"value,omitempty" yaml:"value,omitempty"`
}

func runQSet(cmd *cobra.Command, args []string) {
	data, rest, origin, code, err := qReadSource(cmd, args, false)
	if err != nil {
		qExit(err, code)
		return
	}
	if len(rest) < 1 || len(rest) > 2 {
		exitWithError(cmd, fmt.Errorf("usage: sdt q set <file> <path> <value>"))
		return
	}
	if code, err := qRequireValidJSON(data); err != nil {
		qExit(err, code)
		return
	}
	path := rest[0]
	value, ok := qSetValue(cmd, rest)
	if !ok {
		exitWithError(cmd, fmt.Errorf("missing value (pass it as the third argument or via --input/--inb64/--file)"))
		return
	}

	var out []byte
	if json.Valid([]byte(value)) {
		out, err = sjson.SetRawBytes(data, path, []byte(value))
	} else {
		out, err = sjson.SetBytes(data, path, value)
	}
	if err != nil {
		qExit(fmt.Errorf("set %q: %w", path, err), qExitNotFound)
		return
	}
	qWrite(cmd, out, origin)
}

func runQDel(cmd *cobra.Command, args []string) {
	data, rest, origin, code, err := qReadSource(cmd, args, true)
	if err != nil {
		qExit(err, code)
		return
	}
	if len(rest) != 1 {
		exitWithError(cmd, fmt.Errorf("usage: sdt q del <file> <path>"))
		return
	}
	if code, err := qRequireValidJSON(data); err != nil {
		qExit(err, code)
		return
	}
	path := rest[0]
	if !gjson.GetBytes(data, path).Exists() {
		qExit(fmt.Errorf("path %q not found", path), qExitNotFound)
		return
	}
	out, err := sjson.DeleteBytes(data, path)
	if err != nil {
		qExit(fmt.Errorf("delete %q: %w", path, err), qExitNotFound)
		return
	}
	qWrite(cmd, out, origin)
}

// qSetValue returns the value from the positional argument after the path, or
// from the global input conventions when it is omitted.
func qSetValue(cmd *cobra.Command, rest []string) (string, bool) {
	if len(rest) >= 2 {
		return rest[1], true
	}
	if qGlobalInputSet(cmd) {
		return string(getInputBytes(cmd, nil)), true
	}
	return "", false
}

// qWrite edits a real file in place, preserving its mode; with stdin/global
// input it prints the result so it can be piped.
func qWrite(cmd *cobra.Command, data []byte, origin string) {
	if origin == "" || origin == "-" {
		outputBytes(cmd, data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			outputString(cmd, "\n")
		}
		return
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(origin); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(origin, data, mode); err != nil { //#nosec G306 -- preserves the file's own mode
		qExit(fmt.Errorf("write %s: %w", origin, err), qExitNotFound)
	}
}

func init() {
	qGetCmd.Flags().String("default", "", "Value to print (and exit 0) when the path is absent")
	qCmd.AddCommand(qGetCmd, qSetCmd, qDelCmd)
	rootCmd.AddCommand(qCmd)
}
