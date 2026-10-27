package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// --- regex ---

func TestEditRegexExpandsCaptures(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "port = 8080\n")
	execute(t, editCmd, nil, "-p", rel, "--regex", "--old", `port = ([0-9]+)`, "--new", "PORT=$1")
	if got := mustReadFile(t, abs); got != "PORT=8080\n" {
		t.Errorf("file = %q", got)
	}
}

func TestEditRegexNamedGroupAndDollar(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "ada@example\n")
	execute(t, editCmd, nil, "-p", rel, "--regex", "--old", `(?P<who>[a-z]+)@example`, "--new", `${who}@test`)
	if got := mustReadFile(t, abs); got != "ada@test\n" {
		t.Errorf("file = %q", got)
	}

	abs, rel = editFixture(t, "g.txt", "x\n")
	execute(t, editCmd, nil, "-p", rel, "--regex", "--old", "x", "--new", "$$5")
	if got := mustReadFile(t, abs); got != "$5\n" {
		t.Errorf("$$ expansion = %q", got)
	}
}

func TestEditRegexRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pattern string
		content string
		all     bool
	}{
		{"zero match", "zzz", "abc\n", false},
		{"two matches", "a", "aa\n", false},
		{"uncompilable", "(", "a\n", false},
		{"empty match", "a*", "b\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			abs, rel := editFixture(t, "f.txt", tc.content)
			args := []string{"-p", rel, "--regex", "--old", tc.pattern, "--new", "X"}
			if tc.all {
				args = append(args, "--all")
			}
			shouldExitWithCode(t, 1, func() string { return string(execute(t, editCmd, nil, args...)) })
			if got := mustReadFile(t, abs); got != tc.content {
				t.Errorf("refusal wrote the file: %q", got)
			}
		})
	}
}

func TestEditRegexAllReplacesEveryMatch(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a a a\n")
	execute(t, editCmd, nil, "-p", rel, "--regex", "--old", "a", "--new", "b", "--all")
	if got := mustReadFile(t, abs); got != "b b b\n" {
		t.Errorf("file = %q", got)
	}
}

// --- batch ---

func TestEditBatchAppliesPairsInOrder(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "one two\n")
	execute(t, editCmd, nil, "-p", rel, "--old", "one", "--new", "1", "--old", "two", "--new", "2")
	if got := mustReadFile(t, abs); got != "1 2\n" {
		t.Errorf("file = %q", got)
	}
}

func TestEditBatchPairFailureLeavesTheFile(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "alpha\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "alpha", "--new", "A", "--old", "missing", "--new", "X"))
	})
	if got := mustReadFile(t, abs); got != "alpha\n" {
		t.Errorf("a failed pair wrote the file: %q", got)
	}
}

func TestEditBatchLengthMismatch(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a b\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "a", "--new", "1", "--old", "b"))
	})
	if got := mustReadFile(t, abs); got != "a b\n" {
		t.Errorf("file = %q", got)
	}
}

// --- markers and lines ---

func TestEditMarkersKeepTheBoundaries(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "head\nKEEP\ntail\n")
	execute(t, editCmd, nil, "-p", rel, "--from-marker", "head\n", "--to-marker", "\ntail", "--new", "NEW")
	if got := mustReadFile(t, abs); got != "head\nNEW\ntail\n" {
		t.Errorf("file = %q", got)
	}
}

func TestEditMarkersRefuseMissingAndDuplicated(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"missing", "KEEP\ntail\n"},
		{"duplicated", "head\nx\nhead\nKEEP\ntail\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			abs, rel := editFixture(t, "f.txt", tc.content)
			shouldExitWithCode(t, 1, func() string {
				return string(execute(t, editCmd, nil, "-p", rel, "--from-marker", "head", "--to-marker", "tail", "--new", "X"))
			})
			if got := mustReadFile(t, abs); got != tc.content {
				t.Errorf("file = %q", got)
			}
		})
	}
}

func TestEditLinesReplaceAndDelete(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a\nb\nc\n")
	execute(t, editCmd, nil, "-p", rel, "--lines", "2:2", "--new", "B")
	if got := mustReadFile(t, abs); got != "a\nB\nc\n" {
		t.Errorf("replace = %q", got)
	}

	abs, rel = editFixture(t, "g.txt", "a\nb\nc\n")
	execute(t, editCmd, nil, "-p", rel, "--lines", "2:2", "--new", "")
	if got := mustReadFile(t, abs); got != "a\nc\n" {
		t.Errorf("delete = %q", got)
	}
}

func TestEditLinesRangeRefused(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a\nb\n")
	for _, spec := range []string{"2:1", "0:1", "1:9", "x"} {
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, editCmd, nil, "-p", rel, "--lines", spec, "--new", "X"))
		})
	}
	if got := mustReadFile(t, abs); got != "a\nb\n" {
		t.Errorf("file = %q", got)
	}
}

// --- the block hash ---

func TestEditExpectPinsTheBlock(t *testing.T) {
	sum := sha256.Sum256([]byte("b\n"))
	pin := hex.EncodeToString(sum[:])[:12]

	abs, rel := editFixture(t, "f.txt", "a\nb\nc\n")
	execute(t, editCmd, nil, "-p", rel, "--lines", "2:2", "--new", "B", "--expect", pin)
	if got := mustReadFile(t, abs); got != "a\nB\nc\n" {
		t.Errorf("file = %q", got)
	}
}

func TestEditExpectMismatchLeavesTheFile(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a\nb\nc\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--lines", "2:2", "--new", "B", "--expect", "deadbeef"))
	})
	if got := mustReadFile(t, abs); got != "a\nb\nc\n" {
		t.Errorf("a hash mismatch wrote the file: %q", got)
	}

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--lines", "2:2", "--new", "B", "--expect", "abc"))
	})
}

func TestEditBlockJSONReportsTheHash(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a\nb\nc\n")
	out := execute(t, editCmd, nil, "-p", rel, "--lines", "2:2", "--new", "B", "--dry-run", "--format", "json")
	if !strings.Contains(string(out), `"block_sha256"`) {
		t.Errorf("json result misses block_sha256:\n%s", out)
	}
	_ = abs
}

// --- mode resolution ---

func TestEditRefusesTwoModes(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "a\nb\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--regex", "--lines", "1:1", "--old", "a", "--new", "x"))
	})
	if _, err := os.Stat(abs); err != nil {
		t.Fatal(err)
	}
}
