package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ── LCS-based unified diff ────────────────────────────────────────────────────

// lcs computes the longest common subsequence lengths table.
func lcs(a, b []string) [][]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp
}

// DiffHunk is a single chunk in a unified diff.
type DiffHunk struct {
	OldStart int      `json:"old_start"`
	OldCount int      `json:"old_count"`
	NewStart int      `json:"new_start"`
	NewCount int      `json:"new_count"`
	Lines    []string `json:"lines"`
}

type diffOp struct {
	kind rune // ' ', '-', '+'
	line string
}

type posOp struct {
	kind         rune
	line         string
	aLine, bLine int
}

// lcsBacktrack reconstructs the edit script from the LCS table.
func lcsBacktrack(dp [][]int, aLines, bLines []string) []diffOp {
	var ops []diffOp
	i, j := len(aLines), len(bLines)
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && aLines[i-1] == bLines[j-1] {
			ops = append(ops, diffOp{' ', aLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, diffOp{'+', bLines[j-1]})
			j--
		} else {
			ops = append(ops, diffOp{'-', aLines[i-1]})
			i--
		}
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

// assignLineNumbers converts edit ops into posOps with file-local line numbers.
func assignLineNumbers(ops []diffOp) []posOp {
	aPos, bPos := 0, 0
	out := make([]posOp, 0, len(ops))
	for _, o := range ops {
		var al, bl int
		switch o.kind {
		case ' ':
			aPos++
			bPos++
			al, bl = aPos, bPos
		case '-':
			aPos++
			al = aPos
		case '+':
			bPos++
			bl = bPos
		}
		out = append(out, posOp{o.kind, o.line, al, bl})
	}
	return out
}

// hunkStartLine finds the first non-zero line number for the given side.
func hunkStartLine(chunk []posOp, zeroKind rune, lineField func(posOp) int) int {
	if chunk[0].kind != zeroKind {
		return lineField(chunk[0])
	}
	for _, c := range chunk {
		if v := lineField(c); v > 0 {
			return v
		}
	}
	return 1
}

// emitHunk writes one unified diff hunk to sb.
func emitHunk(sb *strings.Builder, chunk []posOp) {
	aStart := hunkStartLine(chunk, '+', func(p posOp) int { return p.aLine })
	bStart := hunkStartLine(chunk, '-', func(p posOp) int { return p.bLine })
	aCount, bCount := 0, 0
	for _, c := range chunk {
		if c.kind != '+' {
			aCount++
		}
		if c.kind != '-' {
			bCount++
		}
	}
	fmt.Fprintf(sb, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
	for _, c := range chunk {
		fmt.Fprintf(sb, "%c%s\n", c.kind, c.line)
	}
}

// unifiedDiff produces a simplified unified diff between two line slices.
func unifiedDiff(aName, bName string, aLines, bLines []string, context int) string {
	dp := lcs(aLines, bLines)
	posOps := assignLineNumbers(lcsBacktrack(dp, aLines, bLines))

	changed := make([]bool, len(posOps))
	for i, o := range posOps {
		changed[i] = o.kind != ' '
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", aName, bName)

	hunkStart := 0
	for i := 0; i < len(posOps); i++ {
		if !changed[i] {
			continue
		}
		hunkStart = max0(i-context, 0)
		extendTo := i
		for k := i + 1; k < len(posOps) && k <= i+context*2; k++ {
			if changed[k] {
				extendTo = k
			}
		}
		hunkEnd := min0(extendTo+context, len(posOps)-1)
		emitHunk(&sb, posOps[hunkStart:hunkEnd+1])
		i = hunkEnd
	}
	return sb.String()
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min0(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── JSON patch (RFC 6902-style) ───────────────────────────────────────────────

type jsonPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func jsonPatch(aPath, bPath string) ([]jsonPatchOp, error) {
	aData, err := os.ReadFile(aPath) //#nosec G304 -- user-controlled path
	if err != nil {
		return nil, err
	}
	bData, err := os.ReadFile(bPath) //#nosec G304 -- user-controlled path
	if err != nil {
		return nil, err
	}
	var a, b interface{}
	if err := json.Unmarshal(aData, &a); err != nil {
		return nil, fmt.Errorf("file-a is not valid JSON: %w", err)
	}
	if err := json.Unmarshal(bData, &b); err != nil {
		return nil, fmt.Errorf("file-b is not valid JSON: %w", err)
	}
	var ops []jsonPatchOp
	diffJSON("/", a, b, &ops)
	return ops, nil
}

func diffJSON(path string, a, b interface{}, ops *[]jsonPatchOp) {
	switch av := a.(type) {
	case map[string]interface{}:
		bm, ok := b.(map[string]interface{})
		if !ok {
			*ops = append(*ops, jsonPatchOp{Op: cmdReplace, Path: path, Value: b})
			return
		}
		for k, av2 := range av {
			childPath := path + k
			if bv2, exists := bm[k]; exists {
				diffJSON(childPath+"/", av2, bv2, ops)
			} else {
				*ops = append(*ops, jsonPatchOp{Op: "remove", Path: childPath})
			}
		}
		for k, bv2 := range bm {
			if _, exists := av[k]; !exists {
				*ops = append(*ops, jsonPatchOp{Op: "add", Path: path + k, Value: bv2})
			}
		}
	default:
		if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
			p := strings.TrimSuffix(path, "/")
			*ops = append(*ops, jsonPatchOp{Op: cmdReplace, Path: p, Value: b})
		}
	}
}

// ── diff command ─────────────────────────────────────────────────────────────

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare two files and output differences",
	Long: `Compare two files. Supported output formats:
  unified    — standard unified diff (default)
  json-patch — RFC 6902-style JSON patch operations (both inputs must be JSON)`,
	Run: func(cmd *cobra.Command, args []string) {
		aPath := getStringFlag(cmd, "a", false)
		bPath := getStringFlag(cmd, "b", false)
		format := getStringFlag(cmd, "diff-format", false)

		if aPath == "" || bPath == "" {
			exitWithError(cmd, fmt.Errorf("--a and --b are required"))
			return
		}

		switch format {
		case "json-patch":
			ops, err := jsonPatch(aPath, bPath)
			exitWithError(cmd, err)
			out, err := json.MarshalIndent(ops, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)

		default: // unified
			aData, err := os.ReadFile(aPath) //#nosec G304
			exitWithError(cmd, err)
			bData, err := os.ReadFile(bPath) //#nosec G304
			exitWithError(cmd, err)

			aLines := strings.Split(strings.TrimRight(string(aData), "\n"), "\n")
			bLines := strings.Split(strings.TrimRight(string(bData), "\n"), "\n")

			ctx := getIntFlag(cmd, "context", false)
			result := unifiedDiff(aPath, bPath, aLines, bLines, ctx)
			outputString(cmd, result)
		}
	},
}

func init() {
	diffCmd.Flags().String("a", "", "Path to first file")
	diffCmd.Flags().String("b", "", "Path to second file")
	diffCmd.Flags().String("diff-format", "unified", "Output format: unified or json-patch")
	diffCmd.Flags().Int("context", 3, "Lines of context around changes (unified diff only)")
	rootCmd.AddCommand(diffCmd)
}
