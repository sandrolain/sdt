package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ── .env parser ───────────────────────────────────────────────────────────────

// DotEnvEntry represents a single key-value pair in a .env file, preserving order.
type DotEnvEntry struct {
	Key   string
	Value string
}

// parseDotEnv parses a .env file content into an ordered list of key-value pairs.
// Supports:
//   - KEY=VALUE
//   - export KEY=VALUE
//   - # comments (ignored)
//   - quoted values ("value" or 'value')
//   - blank lines (ignored)
func parseDotEnv(content string) ([]DotEnvEntry, error) {
	var entries []DotEnvEntry
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := line[idx+1:]

		// Strip optional inline comment (not inside quotes)
		val = stripInlineComment(val)
		val = unquoteDotEnvValue(val)

		if key == "" {
			continue
		}
		entries = append(entries, DotEnvEntry{Key: key, Value: val})
	}
	return entries, scanner.Err()
}

// stripInlineComment removes an unquoted # comment from a value.
func stripInlineComment(s string) string {
	inSingle, inDouble := false, false
	for i, c := range s {
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble:
			return strings.TrimRight(s[:i], " \t")
		}
	}
	return s
}

func unquoteDotEnvValue(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			s = s[1 : len(s)-1]
		}
	}
	return s
}

// dotEnvToMap converts entries to an ordered JSON object (preserving insertion order via slice).
func dotEnvToMap(entries []DotEnvEntry) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	return m
}

// marshalDotEnvJSON marshals entries as a JSON object in insertion order.
func marshalDotEnvJSON(entries []DotEnvEntry) ([]byte, error) {
	// Build as raw JSON to preserve order
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		kj, err := json.Marshal(e.Key)
		if err != nil {
			return nil, err
		}
		vj, err := json.Marshal(e.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(kj)
		buf.WriteByte(':')
		buf.Write(vj)
	}
	buf.WriteByte('}')
	// pretty-print it
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, buf.Bytes(), "", "  "); err != nil {
		return buf.Bytes(), nil
	}
	return pretty.Bytes(), nil
}

// readDotEnvFile reads and parses a .env file.
func readDotEnvFile(path string) ([]DotEnvEntry, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- user-controlled file path
	if err != nil {
		return nil, err
	}
	return parseDotEnv(string(data))
}

// writeDotEnvFile writes entries back to a file.
func writeDotEnvFile(path string, entries []DotEnvEntry) error {
	var sb strings.Builder
	for _, e := range entries {
		val := e.Value
		if strings.ContainsAny(val, " \t\"'#") {
			val = `"` + strings.ReplaceAll(val, `"`, `\"`) + `"`
		}
		fmt.Fprintf(&sb, "%s=%s\n", e.Key, val)
	}
	return os.WriteFile(path, []byte(sb.String()), 0600)
}

// ── env command ───────────────────────────────────────────────────────────────

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Parse and manipulate .env files",
}

// ── env parse ─────────────────────────────────────────────────────────────────

var envParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse a .env file and output as JSON or shell exports",
	Run: func(cmd *cobra.Command, args []string) {
		file := getStringFlag(cmd, "file", false)
		format := getFormat(cmd)

		var entries []DotEnvEntry
		var err error
		if file != "" {
			entries, err = readDotEnvFile(file)
			exitWithError(cmd, err)
		} else {
			input := getInputString(cmd, args)
			entries, err = parseDotEnv(input)
			exitWithError(cmd, err)
		}

		switch format {
		case "shell":
			for _, e := range entries {
				val := e.Value
				val = strings.ReplaceAll(val, `\`, `\\`)
				val = strings.ReplaceAll(val, `"`, `\"`)
				outputString(cmd, fmt.Sprintf(`export %s="%s"`, e.Key, val))
			}
		default:
			out, err := marshalDotEnvJSON(entries)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		}
	},
}

// ── env get ───────────────────────────────────────────────────────────────────

var envGetCmd = &cobra.Command{
	Use:   "get <KEY>",
	Short: "Get a value from a .env file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := getStringFlag(cmd, "file", true)
		entries, err := readDotEnvFile(file)
		exitWithError(cmd, err)
		key := args[0]
		for _, e := range entries {
			if e.Key == key {
				outputString(cmd, e.Value)
				return
			}
		}
		exitWithError(cmd, fmt.Errorf("key %q not found in %s", key, file))
	},
}

// ── env set ───────────────────────────────────────────────────────────────────

var envSetCmd = &cobra.Command{
	Use:   "set <KEY> <VALUE>",
	Short: "Set or update a value in a .env file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		file := getStringFlag(cmd, "file", true)
		entries, err := readDotEnvFile(file)
		exitWithError(cmd, err)
		key, val := args[0], args[1]
		found := false
		for i, e := range entries {
			if e.Key == key {
				entries[i].Value = val
				found = true
				break
			}
		}
		if !found {
			entries = append(entries, DotEnvEntry{Key: key, Value: val})
		}
		exitWithError(cmd, writeDotEnvFile(file, entries))
		outputString(cmd, "ok")
	},
}

// ── env merge ─────────────────────────────────────────────────────────────────

var envMergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge multiple .env files (last value wins)",
	Run: func(cmd *cobra.Command, args []string) {
		filesStr := getStringFlag(cmd, "files", false)
		output := getStringFlag(cmd, "output", false)

		if filesStr == "" {
			exitWithError(cmd, fmt.Errorf("--files is required"))
		}
		files := strings.Split(filesStr, ",")

		merged := map[string]string{}
		var order []string
		for _, f := range files {
			entries, err := readDotEnvFile(f)
			exitWithError(cmd, err)
			for _, e := range entries {
				if _, exists := merged[e.Key]; !exists {
					order = append(order, e.Key)
				}
				merged[e.Key] = e.Value
			}
		}

		var result []DotEnvEntry
		for _, k := range order {
			result = append(result, DotEnvEntry{Key: k, Value: merged[k]})
		}

		if output != "" {
			exitWithError(cmd, writeDotEnvFile(output, result))
			outputString(cmd, fmt.Sprintf("merged %d keys to %s", len(result), output))
		} else {
			out, err := marshalDotEnvJSON(result)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		}
	},
}

// ── registration ──────────────────────────────────────────────────────────────

func init() {
	envParseCmd.Flags().String("file", "", "Path to .env file (reads stdin if not set)")
	envGetCmd.Flags().String("file", ".env", "Path to .env file")
	envSetCmd.Flags().String("file", ".env", "Path to .env file")
	envMergeCmd.Flags().String("files", "", "Comma-separated list of .env files to merge")
	envMergeCmd.Flags().String("output", "", "Write merged output to file (default: print JSON)")

	envCmd.AddCommand(envParseCmd, envGetCmd, envSetCmd, envMergeCmd)
	rootCmd.AddCommand(envCmd)
}
