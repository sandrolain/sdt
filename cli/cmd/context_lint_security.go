package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// securityPattern is one named regexp class in the opt-in context security scan.
type securityPattern struct {
	name string
	re   *regexp.Regexp
}

// secNamePromptInjection labels every prompt-injection pattern so repeated hits
// collapse into one issue per document.
const secNamePromptInjection = "prompt-injection phrase"

// ctxSecurityPatterns are high-signal patterns for the opt-in
// `sdt context lint --security` scan. The scan never runs in the default lint,
// so historical documents and legitimate security prose do not add noise.
var ctxSecurityPatterns = []securityPattern{
	{secNamePromptInjection, regexp.MustCompile(`(?i)\b(ignore|disregard|forget)\s+(all\s+)?(the\s+)?(previous|prior|above|earlier)\s+(instructions|prompts|rules|context)\b`)},
	{secNamePromptInjection, regexp.MustCompile(`(?i)\b(new|updated)\s+instructions\s*:`)},
	{secNamePromptInjection, regexp.MustCompile(`(?i)\byou\s+are\s+now\b`)},
	{secNamePromptInjection, regexp.MustCompile(`(?i)\b(do\s+not|don't)\s+(tell|inform|reveal|mention)\s+(the\s+)?(user|human|operator)\b`)},
	{secNamePromptInjection, regexp.MustCompile(`(?i)\b(system|developer|jailbreak)\s+(prompt|mode)\b`)},
	{secNamePromptInjection, regexp.MustCompile(`(?i)\bexfiltrat`)},
	{secNamePromptInjection, regexp.MustCompile(`(?i)\boverride\s+(the\s+)?(system|safety|security)\b`)},
	{"private key block", regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`)},
	{"AWS access key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`)},
	{"Slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)},
	{"credential literal", regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|password|passwd|private[_-]?key)\b\s*[:=]\s*["']?[A-Za-z0-9_\-]{20,}["']?`)},
	{"shell pipe to interpreter", regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(ba|z|da)?sh\b`)},
	{"base64 eval", regexp.MustCompile(`(?i)\b(eval|sh\s+-c)\b[^\n]*(base64|atob|b64decode)`)},
	{"reverse shell", regexp.MustCompile(`(?i)(/dev/tcp/|nc\s+-e\b|bash\s+-i\s+>&)`)},
}

// ctxInvisibleRunes are zero-width / bidi-control runes that must not appear in
// knowledge documents: they can hide instructions from human review.
var ctxInvisibleRunes = map[rune]string{
	'\u00ad': "SOFT HYPHEN",
	'\u180e': "MONGOLIAN VOWEL SEPARATOR",
	'\u200b': "ZERO WIDTH SPACE",
	'\u200c': "ZERO WIDTH NON-JOINER",
	'\u200d': "ZERO WIDTH JOINER",
	'\u200e': "LEFT-TO-RIGHT MARK",
	'\u200f': "RIGHT-TO-LEFT MARK",
	'\u202a': "LEFT-TO-RIGHT EMBEDDING",
	'\u202b': "RIGHT-TO-LEFT EMBEDDING",
	'\u202c': "POP DIRECTIONAL FORMATTING",
	'\u202d': "LEFT-TO-RIGHT OVERRIDE",
	'\u202e': "RIGHT-TO-LEFT OVERRIDE",
	'\u2060': "WORD JOINER",
	'\u2066': "LEFT-TO-RIGHT ISOLATE",
	'\u2067': "RIGHT-TO-LEFT ISOLATE",
	'\u2068': "FIRST STRONG ISOLATE",
	'\u2069': "POP DIRECTIONAL ISOLATE",
	'\ufeff': "ZERO WIDTH NO-BREAK SPACE",
}

// lintSecurity scans one document for prompt-injection, credential, invisible
// Unicode and exfil patterns. Findings are always WARNING: the scan is opt-in
// (`--security`), so a hit is a signal the caller explicitly asked for and the
// default lint output is never affected.
func lintSecurity(path string) []ctxLintIssue {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return nil
	}
	content := string(data)
	var issues []ctxLintIssue
	seen := map[string]bool{}
	for _, p := range ctxSecurityPatterns {
		if seen[p.name] {
			continue
		}
		if p.re.MatchString(content) {
			seen[p.name] = true
			issues = append(issues, ctxLintIssue{
				Path:     path,
				Priority: ctxLintWarning,
				Message:  "security: possible " + p.name + " detected",
			})
		}
	}
	if names := invisibleRuneNames(content); len(names) > 0 {
		issues = append(issues, ctxLintIssue{
			Path:     path,
			Priority: ctxLintWarning,
			Message:  "security: invisible/zero-width Unicode present (" + strings.Join(names, ", ") + ")",
		})
	}
	return issues
}

// invisibleRuneNames returns the distinct invisible rune names present in the
// content, with their codepoint, in first-appearance order.
func invisibleRuneNames(content string) []string {
	seen := map[string]bool{}
	var names []string
	for _, r := range content {
		n, ok := ctxInvisibleRunes[r]
		if !ok || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, fmt.Sprintf("U+%04X %s", r, n))
	}
	return names
}

// ctxSourceInstructionFindingCap bounds the per-file findings this walk emits.
// The names are the point when there are a few; a corpus that lost its rename
// would otherwise print thousands of identical lines, so the count survives as
// one finding of its own.
const ctxSourceInstructionFindingCap = 20

// sourceInstructionScan accumulates what the source-tree walk finds.
type sourceInstructionScan struct {
	issues   []ctxLintIssue
	files    int
	dirCount map[string]int
	dirOrder []string
}

// lintSourceInstructionFiles reports the source tree's loadable instruction
// files and the runtimes' configuration directories. It walks context/ingestion/
// and context/refs/ because the corpus scan never visits the immutable sources,
// and it is part of the opt-in `--security` pass only: the default lint is
// unchanged.
//
// A file finding is WARNING when a runtime resolves that name on its own, and
// SUGGESTION for SKILL.md, which only a configured skill directory loads. The
// configuration directories are one finding with their counts rather than one
// per directory: the corpus holds 91 of them at the time of writing, and a line
// per directory would bury the file findings it exists to surface.
func lintSourceInstructionFiles() []ctxLintIssue {
	scan := &sourceInstructionScan{dirCount: map[string]int{}}
	for _, root := range []string{sdtIngestionDir, sdtRefsDir} {
		if walkErr := filepath.WalkDir(root, scan.walk); walkErr != nil {
			continue // an absent or unreadable source root is not a lint failure
		}
	}
	return scan.findings()
}

// walk visits one source-tree entry. An unreadable entry never fails the lint.
func (s *sourceInstructionScan) walk(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	if d.IsDir() {
		return s.enterDir(d.Name())
	}
	s.addFile(path, d.Name())
	return nil
}

// enterDir records a runtime configuration directory and skips its contents: the
// files inside it are agent, command and rule definitions, and counting the
// directory is what keeps the report one line instead of hundreds.
func (s *sourceInstructionScan) enterDir(base string) error {
	if base == ".git" {
		return filepath.SkipDir
	}
	for _, cfg := range sourceInstructionConfigDirs {
		if base == cfg {
			if _, known := s.dirCount[cfg]; !known {
				s.dirOrder = append(s.dirOrder, cfg)
			}
			s.dirCount[cfg]++
			return filepath.SkipDir
		}
	}
	return nil
}

// addFile records one loadable instruction name, up to the cap.
func (s *sourceInstructionScan) addFile(path, base string) {
	file, ok := sourceInstructionFor(base)
	if !ok {
		return
	}
	s.files++
	if s.files > ctxSourceInstructionFindingCap {
		return
	}
	priority := ctxLintWarning
	if !file.loaded {
		priority = ctxLintSuggestion
	}
	s.issues = append(s.issues, ctxLintIssue{
		Path:     corpusRelative(path),
		Priority: priority,
		Message:  fmt.Sprintf("security: third-party instruction file %q under the source tree — an agent runtime loads it as instructions", base),
	})
}

// findings appends the two summary lines: the files beyond the cap and the
// configuration directories, with their counts in discovery order.
func (s *sourceInstructionScan) findings() []ctxLintIssue {
	issues := s.issues
	if s.files > ctxSourceInstructionFindingCap {
		issues = append(issues, ctxLintIssue{
			Path:     sdtRefsDir,
			Priority: ctxLintWarning,
			Message:  fmt.Sprintf("security: third-party instruction files under the source tree: %d (%d shown)", s.files, ctxSourceInstructionFindingCap),
		})
	}
	if len(s.dirOrder) == 0 {
		return issues
	}
	parts := make([]string, 0, len(s.dirOrder))
	for _, cfg := range s.dirOrder {
		parts = append(parts, fmt.Sprintf("%s %d", cfg, s.dirCount[cfg]))
	}
	return append(issues, ctxLintIssue{
		Path:     sdtRefsDir,
		Priority: ctxLintSuggestion,
		Message:  "security: third-party agent configuration directories under the source tree (" + strings.Join(parts, ", ") + ") — their agent, command and rule files are loaded by the matching runtime",
	})
}
