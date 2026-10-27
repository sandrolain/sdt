// Package census classifies the shell commands an agent ran into the shapes the
// adoption analysis measured, so the measurement can be repeated from a file
// instead of re-derived by hand.
//
// It is pure: no I/O, no clock, no database. The caller supplies commands and a
// window; the vocabulary below is the one the analysis's table uses, so a rerun
// compares like with like.
package census

import (
	"regexp"
	"strings"
)

// Shapes is the closed vocabulary, in the analysis's order. It is the single
// source the counts and the report derive from.
// ShapeOther is the fallback for a command outside the measured shapes.
const ShapeOther = "other"

var Shapes = []string{
	"one literal pair",
	"batch of literal pairs",
	"position / slice",
	"whole-body rewrite",
	"whole-file write",
	"sed regex or address",
	"python regex",
	"perl regex",
	"sed literal",
	ShapeOther,
}

var (
	sliceRe = regexp.MustCompile(`\[[^\]]*:[^\]]*\]`)
	sedRe   = regexp.MustCompile(`\bsed\b[^|;&\n]*-i`)
)

// Classify names the shape one command's script belongs to. The order is the
// classifier's contract: a Python heredoc that both batches and writes is a
// batch, because the pairs are the intent and the write is its mechanism.
func Classify(command string) string {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return ShapeOther
	}
	if isPython(cmd) {
		switch {
		case strings.Count(cmd, ".replace(") > 1:
			return "batch of literal pairs"
		case strings.Count(cmd, ".replace(") == 1:
			return "one literal pair"
		case strings.Contains(cmd, "re.sub("):
			return "python regex"
		case strings.Contains(cmd, ".insert("), strings.Contains(cmd, ".index("), sliceRe.MatchString(cmd):
			return "position / slice"
		default:
			return "whole-body rewrite"
		}
	}
	if strings.Contains(cmd, "perl") && (strings.Contains(cmd, "-pi") || strings.Contains(cmd, "-i")) {
		return "perl regex"
	}
	if sedRe.MatchString(cmd) {
		if sedPatternIsRegexp(cmd) {
			return "sed regex or address"
		}
		return "sed literal"
	}
	if strings.Contains(cmd, "cat >") || strings.Contains(cmd, "cat >>") {
		return "whole-file write"
	}
	return ShapeOther
}

func isPython(cmd string) bool {
	for _, name := range []string{"python3", "python ", "python\n", "/python"} {
		if strings.Contains(cmd, name) {
			return true
		}
	}
	return false
}

// sedPatternIsRegexp reads the sed script: a line address before the command
// letter, or a metacharacter in the substitution's pattern, is the
// regex/address shape; a plain literal is the sed-literal shape.
func sedPatternIsRegexp(cmd string) bool {
	script := sedScript(cmd)
	if script == "" {
		return false
	}
	if regexp.MustCompile(`^\s*(\d+(\s*,\s*\d+)?|\$|/[^/]*/)\s*[sdcy!]`).MatchString(script) {
		return true
	}
	for _, line := range strings.Split(script, "\n") {
		i := strings.Index(line, "s/")
		if i < 0 {
			continue
		}
		rest := line[i+2:]
		seg := rest
		if j := strings.Index(rest, "/"); j >= 0 {
			seg = rest[:j]
		}
		if strings.ContainsAny(seg, ".*[]^$()+?|\\") {
			return true
		}
	}
	return false
}

// sedScript extracts the sed program after -i. BSD sed takes an empty backup
// suffix as its own argument (`sed -i ” 's/…/…/'`), so an empty quoted argument
// is skipped rather than mistaken for the program.
func sedScript(cmd string) string {
	i := strings.Index(cmd, "-i")
	if i < 0 {
		return ""
	}
	rest := cmd[i+2:]
	if strings.HasPrefix(rest, ".") {
		if j := strings.IndexAny(rest, " \t"); j >= 0 {
			rest = rest[j+1:]
		} else {
			return ""
		}
	}
	for {
		rest = strings.TrimLeft(rest, " \t\n")
		if rest == "" {
			return ""
		}
		if q := rest[0]; q == '\'' || q == '"' {
			j := strings.IndexByte(rest[1:], q)
			if j < 0 {
				return ""
			}
			if j > 0 {
				return rest[1 : 1+j]
			}
			rest = rest[2+j:]
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return ""
		}
		if fields[0] == "-e" {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "-e"))
			continue
		}
		return fields[0]
	}
}
