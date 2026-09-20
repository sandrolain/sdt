package cmd

import (
	"fmt"
	"os"
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
