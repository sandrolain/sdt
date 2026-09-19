package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var ctxDecisionNumberRegexp = regexp.MustCompile(`^\d{4}$`)

// ctxObjectiveRegexp is the kebab-case slug grammar for the optional
// `objective` grouping key carried by analysis documents.

var ctxObjectiveRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateDecisionNumber(n string) error {
	if !ctxDecisionNumberRegexp.MatchString(n) {
		return fmt.Errorf("decision number must be exactly 4 digits, got %q", n)
	}
	return nil
}

// nextDecisionNumber scans context/decisions/ and returns the 4-digit NNNN that
// follows the highest existing file, or "0001" when the directory is empty or
// does not exist yet.

func nextDecisionNumber() (string, error) {
	entries, err := os.ReadDir(sdtDecisionsDir)
	if os.IsNotExist(err) {
		return "0001", nil
	}
	if err != nil {
		return "", err
	}
	maxN := 0
	for _, e := range entries {
		base := strings.TrimSuffix(e.Name(), sdtMarkdownExt)
		parts := strings.SplitN(base, "-", 2)
		if len(parts) < 2 || len(parts[0]) != 4 {
			continue
		}
		n, nerr := strconv.Atoi(parts[0])
		if nerr != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	return fmt.Sprintf("%04d", maxN+1), nil
}

func contextDecisionFrontmatter(number, title, summary, project, created string) string {
	if summary == "" {
		summary = ctxSummaryPlaceholder
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: decision\n")
	b.WriteString("number: " + number + "\n")
	if title != "" {
		b.WriteString("title: " + yamlScalar(title) + "\n")
	}
	b.WriteString("summary: " + yamlScalar(summary) + "\n")
	b.WriteString("status: proposed\n")
	b.WriteString("created: " + created + "\n")
	b.WriteString("links:\n")
	if project != "" {
		b.WriteString("project: " + yamlScalar(project) + "\n")
	}
	b.WriteString("---\n")
	return b.String()
}

// ── context new ────────────────────────────────────────────────────────────────

// getContextBody returns the file body from --input/--file/--inb64 or piped
// stdin. On a terminal with no input flags it returns "" without blocking.

func getContextBody(cmd *cobra.Command, args []string) string {
	f := cmd.Flags()
	if f.Lookup("input").Changed || f.Lookup("file").Changed || f.Lookup("inb64").Changed {
		return getInputString(cmd, args)
	}
	if stdinIsTTY() {
		return ""
	}
	return getInputString(cmd, args)
}

type contextNewResult struct {
	Path    string `json:"path" yaml:"path"`
	Type    string `json:"type" yaml:"type"`
	Created string `json:"created" yaml:"created"`
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	Status  string `json:"status" yaml:"status"`
}

func outputContextNew(cmd *cobra.Command, res contextNewResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, res.Path+"\n")
	}
}

// yamlScalar quotes s when the plain form would not parse as a valid YAML
// block scalar (e.g. contains ": ", " #", leading/trailing space, newline, or
// starts with a special indicator). Plain form is kept for ordinary values so
// generated frontmatter stays readable.

func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	special := strings.ContainsAny(s, ":#\n\t")
	indicatorPrefix := strings.HasPrefix(s, "-") || strings.HasPrefix(s, "?") ||
		strings.HasPrefix(s, "[") || strings.HasPrefix(s, "]") ||
		strings.HasPrefix(s, "{") || strings.HasPrefix(s, "}") ||
		strings.HasPrefix(s, ",") || strings.HasPrefix(s, "&") ||
		strings.HasPrefix(s, "*") || strings.HasPrefix(s, "!") ||
		strings.HasPrefix(s, "|") || strings.HasPrefix(s, ">") ||
		strings.HasPrefix(s, "%") || strings.HasPrefix(s, "@") ||
		strings.HasPrefix(s, "`") || strings.HasPrefix(s, `"`) ||
		strings.HasPrefix(s, "'")
	if !special && !indicatorPrefix && s == strings.TrimSpace(s) {
		return s
	}
	return strconv.Quote(s)
}

// contextFrontmatter builds the YAML frontmatter for a freshly created
// context/ work file, matching the per-type instruction contracts
// (context/instructions/*.md). summary falls back to ctxSummaryPlaceholder so
// the file stays lint-parseable; title/component are emitted only where the
// type requires them and the value is non-empty.

func contextFrontmatter(typ, title, summary, note, project, component, created, objective string) string {
	if summary == "" {
		summary = ctxSummaryPlaceholder
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: " + typ + "\n")
	if (typ == ctxTypeAnalysis || typ == ctxTypeProposal || typ == ctxTypePrompt || typ == ctxTypeResearch) && title != "" {
		b.WriteString("title: " + yamlScalar(title) + "\n")
	}
	b.WriteString("summary: " + yamlScalar(summary) + "\n")
	if typ == ctxTypeResearch {
		b.WriteString("subject: " + yamlScalar(ctxResearchSubjectPlaceholder) + "\n")
	}
	if note != "" {
		b.WriteString("context: " + yamlScalar(note) + "\n")
	}
	if typ == ctxTypeAnalysis && objective != "" {
		b.WriteString("objective: " + yamlScalar(objective) + "\n")
	}
	if st, ok := ctxDefaultStatus[typ]; ok {
		b.WriteString("status: " + st + "\n")
	}
	if typ == ctxTypeArchitecture && component != "" {
		b.WriteString("component: " + yamlScalar(component) + "\n")
	}
	b.WriteString("created: " + created + "\n")
	if ctxHasUpdated[typ] {
		b.WriteString("updated: " + created + "\n")
	}
	if project != "" {
		b.WriteString("project: " + yamlScalar(project) + "\n")
	}
	b.WriteString("---\n")
	return b.String()
}

func contextDefaultBody(typ string) string {
	switch typ {
	case ctxTypeProposal:
		return "## Problem statement\n\n## Goals\n\n## Non-goals\n\n## Constraints\n\n## Current state\n\n## Proposed design\n\n## Alternatives considered\n\n## Impact and migration\n\n## Validation/evidence\n\n## Decision outcome\n\n## Follow-up\n"
	case ctxTypePrompt:
		return "## Purpose\n\n## Prompt\n\n## Runs\n\n| Date | Model/tool | Scope | Status | Results |\n|---|---|---|---|---|\n"
	case ctxTypeResearch:
		return "## Subject\n\n## Method\n\n## Findings\n\n## Evidence\n\n## Limits and open points\n\n## Feeds\n"
	default:
		return ""
	}
}

var contextNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a context/ work file with frontmatter",
	Long: `Create a plan, analysis, worklog, notes, questions, proposal, prompt,
research, architecture or decision
file under context/ with the correct naming and the full per-type YAML
frontmatter (kind, summary, context, status, created, updated, project plus
per-type fields). The body comes from --input/--file or piped stdin. Existing
files are preserved unless --force is set; --edit opens the file in $EDITOR
after creation.

The slug is derived from --title when --slug is omitted; --summary is optional
and falls back to a MANDATORY-fill placeholder so the file passes lint. For
decision type the next NNNN number is auto-assigned (override with --number).
--objective attaches a kebab-case grouping key (analysis type only). The
command prints the created file path (--format text|json|yaml).

Examples:
  sdt context new --type worklog --title "review deps" --input "reviewed deps"
  sdt context new --type plan --title "ship memory" --force
  sdt context new --type analysis --title "memory backend" --objective memory --input "..."
  sdt context new --type architecture --title "config loading" --summary "config loading component"
  sdt context new --type decision --title "Auth choice" --summary "Use JWT for auth"
	sdt context new --type questions --title "open api questions"
	sdt context new --type proposal --title "add prompt provenance"
	sdt context new --type prompt --title "deepsearch prompt"
	sdt context new --type research --title "deepsearch vector backends"`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		switch typ {
		case ctxTypePlan, ctxTypeAnalysis, ctxTypeWorklog, ctxTypeNotes, ctxTypeQuestions, ctxTypeProposal, ctxTypePrompt, ctxTypeArchitecture, ctxTypeDecision, ctxTypeResearch:
		default:
			exitWithError(cmd, fmt.Errorf("new supports type plan|analysis|worklog|notes|questions|proposal|prompt|architecture|decision|research, got %q", typ))
		}
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		title := getStringFlag(cmd, "title", false)
		if slug == "" && title != "" {
			slug = sanitizeSlug(title)
		}
		if (typ == ctxTypeArchitecture || typ == ctxTypeDecision || typ == ctxTypeProposal || typ == ctxTypePrompt || typ == ctxTypeResearch) && slug == "" {
			exitWithError(cmd, fmt.Errorf("--title or --slug is required for type %s", typ))
		}
		note := getStringFlag(cmd, "context", false)
		summary := getStringFlag(cmd, "summary", false)
		objective := getStringFlag(cmd, "objective", false)
		if objective != "" {
			if typ != ctxTypeAnalysis {
				exitWithError(cmd, fmt.Errorf("--objective is only supported for --type analysis, got %q", typ))
			}
			if !ctxObjectiveRegexp.MatchString(objective) {
				exitWithError(cmd, fmt.Errorf("--objective must be a kebab-case slug (lowercase alphanumeric and '-'), got %q", objective))
			}
		}
		force := getBoolFlag(cmd, "force", false)
		edit := getBoolFlag(cmd, "edit", false)
		numberOverride := getStringFlag(cmd, "number", false)
		body := getContextBody(cmd, args)
		created := contextNow().UTC().Format(time.RFC3339)

		project := ""
		if cfg, err := findProjectConfig(); err == nil && cfg != nil {
			project = cfg.Project
		}

		var path string
		var content string
		if typ == ctxTypeDecision {
			decNum := numberOverride
			if decNum == "" {
				var numerr error
				decNum, numerr = nextDecisionNumber()
				exitWithError(cmd, numerr)
			}
			if err := validateDecisionNumber(decNum); err != nil {
				exitWithError(cmd, err)
			}
			path = filepath.Join(sdtDecisionsDir, decNum+"-"+slug+".md")
			content = contextDecisionFrontmatter(decNum, title, summary, project, created)
		} else {
			var err error
			path, err = contextPath(typ, slug, "", "")
			exitWithError(cmd, err)
			component := ""
			if typ == ctxTypeArchitecture {
				component = slug
			}
			content = contextFrontmatter(typ, title, summary, note, project, component, created, objective)
			if body == "" {
				body = contextDefaultBody(typ)
			}
		}

		status := statusCreated
		if _, err := os.Stat(path); err == nil {
			if !force {
				exitWithError(cmd, fmt.Errorf("%s already exists (use --force to overwrite)", path))
			}
			status = statusUpdated
		} else if !os.IsNotExist(err) {
			exitWithError(cmd, err)
		}
		if body != "" {
			content += "\n" + strings.TrimRight(body, "\n") + "\n"
		}

		dir, _ := filepath.Split(path)
		if err := os.MkdirAll(dir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}

		if edit {
			if err := contextRunEditor(path); err != nil {
				exitWithError(cmd, err)
			}
		}

		outputContextNew(cmd, contextNewResult{
			Path:    path,
			Type:    typ,
			Created: created,
			Project: project,
			Status:  status,
		})
	},
}

// ── context list ───────────────────────────────────────────────────────────────
