package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ── closed role register ───────────────────────────────────────────────────────

// roleDescriptor is one entry of the closed role register: the canonical slug,
// a human title, the coordinator flag and the default owned path globs. The
// owned globs cover the conventional context/ document spaces a role owns in
// any SDT project; repo code paths are project-specific and are derived from
// repo evidence by the project layer (Wave 3 phase 4), never invented here.
// The register is the single source of truth for profile generation, the
// `agent roles` surfaces and the `role:` frontmatter vocabulary.

type roleDescriptor struct {
	Slug        string   `json:"slug" yaml:"slug"`
	Title       string   `json:"title" yaml:"title"`
	Coordinator bool     `json:"coordinator" yaml:"coordinator"`
	Owned       []string `json:"owned,omitempty" yaml:"owned,omitempty"`
}

// roleRegister is the closed set of role slugs. Order matters: pm (the
// coordinator) first, then the execution/review roles in canonical order. The
// set is curated; adding a slug is a deliberate change (see roles check).

var roleRegister = []roleDescriptor{
	{Slug: roleSlugPM, Title: "Project manager", Coordinator: true, Owned: []string{"context/plan", "context/tasks"}},
	{Slug: roleSlugBackend, Title: "Backend engineer"},
	{Slug: roleSlugFrontend, Title: "Frontend engineer"},
	{Slug: roleSlugArchitect, Title: "Solution architect", Owned: []string{"context/architecture", "context/decisions", "context/analysis"}},
	{Slug: roleSlugReviewer, Title: "Reviewer", Owned: []string{"context/proposals", "context/questions"}},
	{Slug: roleSlugQA, Title: "Quality assurance"},
	{Slug: roleSlugDevops, Title: "DevOps engineer"},
	{Slug: cmdDocs, Title: "Documentation engineer", Owned: []string{"context/wiki", "context/instructions", "context/research"}},
}

func roleLookup(slug string) (roleDescriptor, bool) {
	for _, r := range roleRegister {
		if r.Slug == slug {
			return r, true
		}
	}
	return roleDescriptor{}, false
}

func roleSlugs() []string {
	slugs := make([]string, 0, len(roleRegister))
	for _, r := range roleRegister {
		slugs = append(slugs, r.Slug)
	}
	return slugs
}

func coordinatorSlug() string {
	for _, r := range roleRegister {
		if r.Coordinator {
			return r.Slug
		}
	}
	return ""
}

// ── agent roles command group ──────────────────────────────────────────────────

var agentRolesCmd = &cobra.Command{
	Use:   doctorNameRoles,
	Short: "Role register and generated role profiles",
	Long: `Manage the closed role register and the generated role profiles under
` + "`context/roles/`" + `:

  roles show    list the closed role register
  roles init    generate the role profiles under context/roles/
  roles check   validate the profile set, drift and owned-path overlap
`,
}

var agentRolesShowCmd = &cobra.Command{
	Use:   useShow,
	Short: "List the closed role register",
	Long: `Render the closed role register: every canonical role slug, its title,
the coordinator flag and the default owned path globs. The register is the
single source of truth for the ` + "`role:`" + ` frontmatter vocabulary.
`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(roleRegister, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(roleRegister)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, r := range roleRegister {
				line := fmt.Sprintf("%s %-10s %s", coordMark(r), r.Slug, r.Title)
				if len(r.Owned) > 0 {
					line += " — owned: " + strings.Join(r.Owned, ", ")
				}
				outputString(cmd, line+"\n")
			}
			outputString(cmd, fmt.Sprintf("\n%d role(s); * marks the coordinator\n", len(roleRegister)))
		}
	},
}

// coordMark returns a one-char marker for the coordinator row in the text
// rendering of the register.

func coordMark(r roleDescriptor) string {
	if r.Coordinator {
		return "*"
	}
	return " "
}

var agentRolesInitCmd = &cobra.Command{
	Use:   useInit,
	Short: "Generate role profiles under context/roles/",
	Long: `Generate the base-first three-layer role profiles under
` + "`context/roles/<slug>.md`" + ` plus the shared rules file
` + "`context/roles/shared.md`" + `.

Non-destructive by default: existing profiles are preserved. Use --force to
refresh the generated core and project layers in place — the user-owned
preferences scope is never overwritten. The project layer is derived from repo
evidence (stack, layout, real build/test/lint commands); use --ask to answer
the ≤3 non-derivable role facts interactively. Project identity comes from
--project, else .sdt.yaml.

Examples:
  sdt agent roles init
  sdt agent roles init --force
  sdt agent roles init --ask
  sdt agent roles init --project myapp --force`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		force := getBoolFlag(cmd, "force", false)
		ask := getBoolFlag(cmd, "ask", false)
		project := getStringFlag(cmd, "project", false)
		if project == "" {
			if cfg, err := findProjectConfig(); err == nil && cfg != nil {
				project = cfg.Project
			}
		}
		facts := deriveRoleProjectFacts()
		if ask {
			facts = runRoleQuestionnaire(cmd, facts)
		}
		outputFileResults(cmd, writeRoleFiles(project, force, facts))
	},
}

func init() {
	agentRolesInitCmd.Flags().Bool("force", false, "Refresh generated core/project layers (user preferences preserved)")
	agentRolesInitCmd.Flags().Bool("ask", false, "Ask up to 3 non-derivable role facts (interactive)")
	agentRolesInitCmd.Flags().String("project", "", "Project id (default: from .sdt.yaml)")

	agentRolesCmd.AddCommand(agentRolesShowCmd, agentRolesInitCmd, agentRolesCheckCmd)
	agentCmd.AddCommand(agentRolesCmd)
}

// runRoleQuestionnaire collects non-derivable role facts from the user (bounded
// ≤3 questions) when --ask is set and stdin is a terminal. agentPrompt already
// returns the default in non-interactive runs, so the questionnaire is a no-op
// in CI/tests and never blocks.

func runRoleQuestionnaire(cmd *cobra.Command, f roleProjectFacts) roleProjectFacts {
	if !getBoolFlag(cmd, "ask", false) || !stdinIsTTY() {
		return f
	}
	answers := make([]string, 0, len(roleQuestionnaireItems))
	for _, q := range roleQuestionnaireItems {
		answers = append(answers, agentPrompt(cmd, false, q.label, ""))
	}
	return applyRoleAnswers(f, answers)
}
