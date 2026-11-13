//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// materialReviewCmd records a reader verdict in the ledger beside a material:
// the verdict text, the reviewer session, the model and the date land in
// cycle.reader (plus reader_by / reader_model / reader_at), which
// sdt material status reports as "recorded". The last verdict wins; earlier
// ones are replaced, not appended. The ledger is edited surgically: every byte
// outside the cycle object stays as it is.
var materialReviewCmd = &cobra.Command{
	Use:   "review <concept> --verdict <text>",
	Short: "Record a reader verdict in a material's ledger",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		role := strings.TrimSpace(getStringFlag(cmd, "role", false))
		if role == "" {
			role = "reader"
		}
		if role != "reader" && role != "critic" {
			exitWithError(cmd, fmt.Errorf("--role must be reader or critic, got %q", role))
			return
		}
		verdict := strings.TrimSpace(getStringFlag(cmd, "verdict", false))
		if verdict == "" {
			exitWithError(cmd, fmt.Errorf("--verdict is required: the verdict is the review; empty means not run"))
			return
		}
		ref, err := resolveMaterialFile(args[0], getStringFlag(cmd, "syllabus", false), getStringFlag(cmd, "lang", false))
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		by := strings.TrimSpace(getStringFlag(cmd, "by", false))
		model := strings.TrimSpace(getStringFlag(cmd, "model", false))
		ledgerPath := materialLedgerPathOf(ref.Path)
		if err := materialLedgerRecordReader(ledgerPath, role, verdict, by, model, time.Now().UTC().Format(time.RFC3339)); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, fmt.Sprintf("recorded %s verdict for %s (%s)\nverdict: %s\n",
			role, strings.TrimPrefix(ref.Header.Concept, "wiki/"), filepath.ToSlash(ledgerPath), verdict))
	},
}

// materialLedgerPathOf derives the ledger path beside a material file:
// <base>.ledger.json, where base is the file name without its .md suffix.
func materialLedgerPathOf(materialPath string) string {
	return strings.TrimSuffix(materialPath, ".md") + ".ledger.json"
}

// materialLedgerRecordReader replaces the cycle object's role verdict in a
// ledger, byte-surgically: the text outside the cycle object is untouched, the
// fields keep the file's indentation, and the other role's value is carried
// unchanged. A ledger without a cycle object is refused rather than invented.
func materialLedgerRecordReader(path, role, verdict, by, model, at string) error {
	raw, err := os.ReadFile(path) //#nosec G304,G703 -- the ledger beside the resolved material
	if err != nil {
		return fmt.Errorf("read ledger %s: %w", filepath.ToSlash(path), err)
	}
	other, err := materialLedgerCriticOf(string(raw), role)
	if err != nil {
		return fmt.Errorf("ledger %s: %w", filepath.ToSlash(path), err)
	}
	out, err := materialLedgerReplaceCycle(string(raw), role, other, verdict, by, model, at)
	if err != nil {
		return fmt.Errorf("ledger %s: %w", filepath.ToSlash(path), err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil { //#nosec G306,G703 -- the lane's own ledger, beside the resolved material
		return fmt.Errorf("write ledger %s: %w", filepath.ToSlash(path), err)
	}
	return nil
}

// materialLedgerCriticOf reads the other role's value from the cycle object so
// the splice carries it unchanged (recording the reader carries the critic,
// and vice versa).
func materialLedgerCriticOf(text, recorded string) (string, error) {
	block, err := materialLedgerCycleBlock(text)
	if err != nil {
		return "", err
	}
	var cycle struct {
		Critic string `json:"critic"`
		Reader string `json:"reader"`
	}
	if err := json.Unmarshal([]byte(block), &cycle); err != nil {
		return "", fmt.Errorf("parse cycle: %w", err)
	}
	if recorded == "reader" {
		return cycle.Critic, nil
	}
	return cycle.Reader, nil
}

// materialLedgerCycleBlock returns the cycle object's JSON text, located by its
// `"cycle"` key. The scan tracks brace depth outside string literals, so a
// value containing braces cannot break it.
func materialLedgerCycleBlock(text string) (string, error) {
	key := strings.Index(text, "\"cycle\"")
	if key < 0 {
		return "", fmt.Errorf("no \"cycle\" object: regenerate the ledger beside the material first")
	}
	open := strings.Index(text[key:], "{")
	if open < 0 {
		return "", fmt.Errorf("the \"cycle\" key carries no object")
	}
	open += key
	depth, inString, escaped := 0, false, false
	for i := open; i < len(text); i++ {
		switch c := text[i]; {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return text[open : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("the \"cycle\" object never closes")
}

// materialLedgerReplaceCycle returns the ledger with its cycle object replaced
// by one carrying critic, reader and, when given, the role's _by/_at keys and
// reader_model. The replacement keeps the object's own indentation.
func materialLedgerReplaceCycle(text, role, other, verdict, by, model, at string) (string, error) {
	block, err := materialLedgerCycleBlock(text)
	if err != nil {
		return "", err
	}
	indent := materialLedgerIndentOf(text, block)
	critic, reader := other, other
	byKey, atKey := "reader_by", "reader_at"
	if role == "critic" {
		critic, byKey, atKey = verdict, "critic_by", "critic_at"
	} else {
		reader = verdict
	}
	fields := []struct{ key, value string }{
		{"critic", critic},
		{"reader", reader},
	}
	if by != "" {
		fields = append(fields, struct{ key, value string }{byKey, by})
	}
	if model != "" {
		fields = append(fields, struct{ key, value string }{"reader_model", model})
	}
	if at != "" {
		fields = append(fields, struct{ key, value string }{atKey, at})
	}
	var replaced strings.Builder
	replaced.WriteString("{")
	for i, f := range fields {
		encoded, err := json.Marshal(f.value)
		if err != nil {
			return "", err
		}
		if i > 0 {
			replaced.WriteString(",")
		}
		replaced.WriteString("\n" + indent + "\"" + f.key + "\": " + string(encoded))
	}
	replaced.WriteString("\n" + strings.TrimSuffix(indent, " ") + "}")
	return strings.Replace(text, block, replaced.String(), 1), nil
}

// materialLedgerIndentOf infers the indentation one level inside the cycle
// object from the line that opens it, so the splice matches the file.
func materialLedgerIndentOf(text, block string) string {
	blockStart := strings.Index(text, block)
	lineStart := strings.LastIndex(text[:blockStart], "\n") + 1
	line := text[lineStart:blockStart]
	base := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	return base + "  "
}

func init() {
	materialReviewCmd.Flags().String("verdict", "", "the reader verdict (required)")
	materialReviewCmd.Flags().String("role", "", "which cycle pass records: reader (default) or critic")
	materialReviewCmd.Flags().String("by", "", "the reviewer session id")
	materialReviewCmd.Flags().String("model", "", "the model the reviewer ran on")
	materialReviewCmd.Flags().String("syllabus", "", "syllabus to disambiguate, with or without syllabi/")
	materialReviewCmd.Flags().String("lang", "", "language to disambiguate, e.g. it")
	materialCmd.AddCommand(materialReviewCmd)
}
