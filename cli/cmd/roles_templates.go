package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── role profile templates (role core + shared rules file) ────────────────────
//
// Profile generation is base-first: the role core is the stable, project-agnostic
// layer; the project layer and the user preferences are separate marker scopes
// that regeneration (--force) and the user own respectively. Every layer marker
// is scoped `roles/<slug>/<layer>` so the existing marker-merge machinery can
// refresh a single layer. The shared rules file (context/roles/shared.md) holds
// the rules common to every profile; profiles reference it instead of restating
// it (no duplication).

// sdtRolesSharedFile is the shared-rules document every role profile references.
const sdtRolesSharedFile = "shared.md"

// roleLayerCore / roleLayerProject / roleLayerPrefs are the three marker scopes
// per profile, in generation order. The preferences scope is user-owned and is
// never rewritten by --force (see agentWriteRoleFile in the init phase).

const (
	roleLayerCore        = "core"
	roleLayerProject     = "project"
	roleLayerPreferences = "preferences"
)

// roleSectionName returns the marker name for a role profile layer.
func roleSectionName(role, layer string) string {
	return filepath.Join(filepath.Base(sdtRolesDir), role, layer)
}

// ── shared rules file ─────────────────────────────────────────────────────────

// instrRoleSharedTemplate renders context/roles/shared.md from the closed
// register so the role table never drifts from the Go constant.
func instrRoleSharedTemplate() string {
	var table strings.Builder
	table.WriteString("| slug | title | coordinator | owned (default context/ spaces) |\n")
	table.WriteString("|---|---|---|---|\n")
	for _, r := range roleRegister {
		coord := "no"
		if r.Coordinator {
			coord = "yes"
		}
		owned := "—"
		if len(r.Owned) > 0 {
			owned = "`" + strings.Join(r.Owned, "`, `") + "`"
		}
		fmt.Fprintf(&table, "| `%s` | %s | %s | %s |\n", r.Slug, r.Title, coord, owned)
	}

	return `# Shared Role Rules

**Every profile in ` + "`context/roles/`" + ` references this file. Read it before
any role profile.** It is the shared layer: rules common to all roles live here
once, profiles never restate them. Generated from the same closed register the
CLI uses, so the table below cannot drift from ` + "`sdt agent roles show`" + `.

## Role register (closed)

The set is **closed and curated**: adding a role is a deliberate change that
touches the register, the templates and ` + "`roles check`" + `. ` + "`pm`" + ` is the
coordinator: it owns planning/sequencing and handoffs over the seven
execution/review roles.

` + table.String() + `

Every role owns a default set of ` + "`context/`" + ` document spaces (column
"owned"); repo code paths are added by the project layer, never invented here.
Two roles must never own the same path: where work areas touch, one role owns
and another reviews (` + "`roles check`" + ` enforces owner/reviewer pairing).

## Three layers per profile

Each profile is three marker-delimited layers:

1. **` + "`core`" + `** — role-generic: mission, responsibilities, working mode,
   handoff, definition of done, anti-patterns. Stable, generated once.
2. **` + "`project`" + `** — role-relevant project facts: stack, repo layout, real
   build/test/lint commands, conventions with concrete paths. Derived from repo
   evidence or answered by the user (≤3 targeted questions). Regenerated
   independently by ` + "`sdt agent roles init --force`" + `.
3. **` + "`preferences`" + `** — user-owned. ` + "`--force`" + ` never overwrites it;
   only the user edits it. Keep it short and role-relevant.

Layers are additive: a repo change refreshes only the project layer, never the
core, and never user preferences.

## Source tags and assumptions

Generated lines in the project layer are tagged with their source:

- ` + "`(repo)`" + ` — derived from repo evidence (AGENTS.md project block,
  project.md, real commands, layout).
- ` + "`(user)`" + ` — answered by the user.
- ` + "`(assumption)`" + ` — not derivable and not asked; a best-effort guess.

Never invent: a fact that cannot be derived becomes a tagged ` + "`(assumption)`" + `
line, **never** a silent rule. Every assumption is surfaced in the profile's
"Open assumptions" footer, kept there until resolved with evidence.

## Capability contract (advisory)

A profile's "Responsibilities / non-responsibilities" is a **declared contract**,
not a sandbox: SDT cannot enforce it inside third-party runtimes. Enforcement is
two deterministic checks — closed register and owner/reviewer overlap — plus
honest discipline. Escalation order is ` + "`job > plan > global`" + `: a task-level
instruction wins over the plan, which wins over this shared layer.

## Handoff protocol

Every deliverable crosses roles with a defined format (each profile's 9-point
structure, point 7):

- who receives, who delivers, in what format (` + "`context/decisions/NNNN-<slug>.md`" + `,
  task-file ` + "`## Review`" + ` block, worklog entry, ...)
- owner/reviewer pairs: the reviewer never self-approves (independent pass).
- a handoff is complete only when the receiving role acknowledges it in the same
  format it was delivered.

## Verification vocabulary

Use the standardized claims: **passed** (ran and verified), **expected**
(written, not run), **inferred** (static analysis only). Close phases with the
verify-step and record verdicts in the task-file ` + "`## Review`" + ` block
(CONFIRMED / DISPROVED / UNVERIFIED). Deterministic gates: ` + "`sdt agent verify`" + `,
` + "`sdt agent doctor`" + `, ` + "`sdt agent gate`" + `.

## Profile body rules

- **No H1** — bodies start at H2 (AGENTS.md document conventions).
- ≤ ~80 lines per core; split overflow into per-role ref files loaded on demand.
- Reference ` + "`shared.md`" + `, AGENTS.md and the project layer instead of
  restating them.
- Every claim is verifiable or actionable; anchor rules to real paths/commands.
- Profiles are advisory content: they shape behaviour, they do not grant
  permissions.
`
}

// ── role core templates ────────────────────────────────────────────────────────

// roleCoreContent is the authored, role-generic body per role. Project-dependent
// points (3 context, 4 conventions, 5 preferences) stay in the project/preferences
// layers; the core points them there instead of duplicating project facts.

type roleCoreContent struct {
	mission          string
	responsibilities string
	nonResp          string
	workingMode      string
	handoff          string
	dod              string
	antiPatterns     string
}

// roleCoreContents keys the authored core bodies by the closed-register slugs.
// A missing key is a test failure: every register slug must have a template.

var roleCoreContents = map[string]roleCoreContent{
	roleSlugPM: {
		mission:          "Coordinate the plan → tasks → execution lifecycle and the seven execution/review roles, so work is sequenced, handoffs are unambiguous and phases close with evidence.",
		responsibilities: "- own planning and sequencing: analysis → plan → task files, one phase at a time\n- keep open questions answered or registered in `context/questions/`\n- enforce handoff and closeout discipline (task `## Review` blocks, worklog crystallization)\n- keep task files and the plan accurate while phases run (update in place).",
		nonResp:          "- never writes code or specs that belong to backend/frontend/architect\n- never self-approves a review (independent pass only)\n- never edits past analyses/plans on its own initiative.",
		workingMode:      "- before planning: read `context/index.md`, the essential tier and the relevant analyses\n- one phase in progress at a time; mark task files in progress on take-in\n- ask the user for every open point; register unanswered ones in `context/questions/`\n- propose, then let the user decide style/architecture and scope.",
		handoff:          "- receives: open questions and completed phases from every role; user decisions\n- delivers: task files, plan updates, decision requests to the user, phase closeouts\n- format: task-file `## Review` blocks and worklog entries, decisions as `context/decisions/NNNN-<slug>.md`.",
		dod:              "- phases close only after the verify-step verdicts are recorded\n- task files and the plan reflect reality; no stale `[~]` markers\n- `sdt context reindex`/`lint`/`status` clean; `sdt agent verify` passes.",
		antiPatterns:     "- filing a plan before the analysis exists, or task files before plan approval\n- leaving `[~]` items unattended across sessions\n- rewriting past documents instead of appending/updating in place.",
	},
	roleSlugBackend: {
		mission:          "Own the server-side implementation: APIs, persistence, business logic and the CLI surface, delivered in reviewable increments.",
		responsibilities: "- implement and own backend code, its tests (≥80% per package) and its commands\n- keep the declared language/toolchain discipline (e.g. no CGO when the project declares pure-Go)\n- surface code patterns that should become conventions (write them back in the project layer).",
		nonResp:          "- does not decide architecture unilaterally (architect owns design authority)\n- does not ship changes without their tests, unless explicitly asked\n- does not rewrite frontend or docs-owned surfaces.",
		workingMode:      "- follow the conventions from the project layer and the repo's real commands\n- run the project's build/test/lint before marking anything done\n- escalate ambiguous requirements to the pm rather than guessing",
		handoff:          "- receives: ADRs/design from architect, phased tasks from pm\n- delivers: implemented+tested code to reviewer (independent review), release-ready builds to devops\n- format: mergeable commits (Conventional Commits) + task-file completion notes.",
		dod:              "- the project's test and lint commands pass locally; per-package coverage ≥80%\n- no issues from the declared linter\n- handoff includes what changed and how it was verified.",
		antiPatterns:     "- inventing architecture instead of implementing the ADR\n- shipping without running the declared test/lint commands\n- adding dependencies without the library-first gate.",
	},
	roleSlugFrontend: {
		mission:          "Own the web/UI surface: components, state, styling and the build, delivered in reviewable increments consistent with the design system.",
		responsibilities: "- implement and own frontend code, its tests (jsdom/vitest when declared) and the bundler setup\n- follow the project's design tokens/design system instead of ad-hoc styles\n- surface reusable component patterns in the project layer.",
		nonResp:          "- does not restyle/rewrite backend-owned documents or API contracts\n- does not bypass the declared test runner or linter\n- does not introduce build dependencies without the library-first gate.",
		workingMode:      "- read the project layer for stack (Vite/React/bun etc.), lint/test commands and design tokens\n- run the declared lint + tests before completion\n- ask the pm before churning shared components used by multiple surfaces.",
		handoff:          "- receives: interface specs/ADRs from architect, tasks from pm\n- delivers: implemented+tested UI to reviewer, bundled assets to devops\n- format: commits + concise completion notes with before/after when visual.",
		dod:              "- declared lint and test suites pass; build succeeds\n- no drift from design tokens; new components follow existing patterns\n- handoff states what changed and how to verify.",
		antiPatterns:     "- inventing a second design system instead of using the tokens\n- committing untested components or bypassing the bundler's lint\n- restyling documents that belong to the docs role.",
	},
	"architect": {
		mission:          "Own design authority: turn problems and requirements into explicit, reviewable architecture and decision records the other roles can implement.",
		responsibilities: "- produce and maintain ADRs (`context/decisions/NNNN-<slug>.md`) and living architecture docs (`context/architecture/`)\n- resolve structure, boundaries and patterns **with the user** before implementation (style & architecture agreed a priori)\n- run the library-first evaluation (web search + local docs) and present the shortlist to the user.",
		nonResp:          "- does not implement the code itself (backend/frontend do)\n- does not decree architecture without the user's approval\n- does not restate shared rules already in `shared.md` or the project layer.",
		workingMode:      "- design first, implement never: propose options with trade-offs, let the user decide\n- trace decisions: ADR → architecture doc → implementation, bidirectionally referenceable\n- keep decisions read-only after acceptance; supersede, never edit in place.",
		handoff:          "- receives: requirements and constraints from pm/user; review feedback on decisions\n- delivers: ADRs to backend/frontend/devops/docs, decision updates to pm\n- format: `context/decisions/NNNN-<slug>.md` + architecture pages.",
		dod:              "- every non-trivial design choice is user-approved before implementation starts\n- ADRs/architecture stay consistent with the codebase (no uncodified drift)\n- library choices recorded with why + alternative.",
		antiPatterns:     "- choosing a \"reasonable default\" instead of asking the user\n- letting architecture drift from its own ADRs\n- reinventing a library the ecosystem already provides.",
	},
	"reviewer": {
		mission:          "Provide the independent second pass: verify phases and deliverables against their claims, using evidence, never self-approval.",
		responsibilities: "- review deliverables (code, docs, phases) against the task-file claims and the project's gates\n- run the verify-step and record CONFIRMED / DISPROVED / UNVERIFIED verdicts\n- own the review surface: proposals and open questions get closed verdicts.",
		nonResp:          "- does not implement the work it reviews\n- does not rubber-stamp; every verdict needs evidence (command output, file:line)\n- does not enforce rules that need a runtime SDT cannot provide (advisory-only).",
		workingMode:      "- independent pass: the author of the work does not self-approve\n- degrade gracefully: CRITICAL before WARNING before SUGGESTION\n- reproduce claims with the real commands before confirming them.",
		handoff:          "- receives: completed code/phases from executor roles, proposals from the proposer\n- delivers: review verdicts to pm and the authors\n- format: task-file `## Review` blocks and proposal verdicts (accepted/rejected).",
		dod:              "- each finding ends in a closed verdict with evidence\n- review gate passes (`sdt agent gate` when a hard gate is wanted)\n- findings are actionable: hint + remediation next action.",
		antiPatterns:     "- approving without running the commands (claims marked passed must be run)\n- reviewing one's own work\n- flagging without a remediation hint.",
	},
	"qa": {
		mission:          "Own verification beyond unit tests: integration, end-to-end and acceptance checks that catch what the author missed.",
		responsibilities: "- design and run test scenarios against the project's real build/test commands\n- own the acceptance lens: does the deliverable meet the definition of done in its plan/task\n- report failures with reproduction steps, not just symptoms.",
		nonResp:          "- does not fix the code it tests (report it to the owner)\n- does not redefine the definition of done unilaterally (pm owns closeouts)\n- does not run checks that need a runtime the project does not declare.",
		workingMode:      "- test from the deliverable's own claims (definition of done)\n- run the declared gate commands before marking a check passed\n- escalate a fail to the owner + pm with reproduction evidence.",
		handoff:          "- receives: deliverables to be tested from backend/frontend, acceptance criteria from pm\n- delivers: test reports and fail reproductions to owners and pm\n- format: worklog/notes entries with commands run and results.",
		dod:              "- every acceptance criterion is checked or explicitly waived by the pm\n- failures are reported with a reproduction path\n- verdicts use the standardized claim vocabulary.",
		antiPatterns:     "- testing only happy paths\n- waiving criteria without the pm's sign-off\n- claiming passed for checks that were not run.",
	},
	roleSlugDevops: {
		mission:          "Own the build, release and operational surfaces: reproducible builds, pipelines, config and the tooling the other roles rely on.",
		responsibilities: "- own build/CI/release automation and the project's declared commands (Taskfile/Makefile/test/lint)\n- keep dependency and security hygiene (vuln scans when declared: govulncheck, trivy)\n- keep the run/build documentation in sync with reality.",
		nonResp:          "- does not change application behaviour that belongs to backend/frontend\n- does not add repo-wide tooling without telling the affected roles\n- does not hide build steps in undocumented custom scripts.",
		workingMode:      "- prefer the repo's declared task definitions (Taskfile/Makefile/package.json scripts)\n- document every operational rule with its command\n- surface environment/version constraints as facts (`.tool-versions`, go.mod).",
		handoff:          "- receives: release-ready builds from backend/frontend, infra needs from the user\n- delivers: pipelines, releases and operational docs to all roles\n- format: versioned releases + runnable instructions in the project layer.",
		dod:              "- the declared build/test/lint/latest gate commands work from a clean checkout\n- no secrets in repos/pipelines; vuln scan clean or documented exceptions\n- operations documented where the roles look (project layer, wiki).",
		antiPatterns:     "- non-reproducible builds (works on my machine)\n- adding infra/runtime assumptions without tagging them `(assumption)`\n- leaving build steps only in a custom script instead of the declared task system.",
	},
	cmdDocs: {
		mission:          "Own the knowledge surface: wiki, instructions and research — so the project's understanding outlives any single conversation.",
		responsibilities: "- own `context/wiki/`, `context/instructions/` and `context/research/` content\n- keep documents current against the code, and correlated: explicit relations, provenance `sources`, dedup before write\n- record lessons and decision-log entries as reality emerges.",
		nonResp:          "- does not duplicate shared rules (reference `shared.md` / instructions)\n- does not backfill `role:`/`agent:` into historical entries\n- does not convert formats without the ingestion contract (anydoc first, docling fallback).",
		workingMode:      "- write for the reader: concise technical language, no invented conventions\n- correlate before writing: search first, link `[[wiki/<path>]]`, keep `sources` resolvable\n- file substantive answers back into `context/` — never leave knowledge in chat.",
		handoff:          "- receives: features/commands from all roles to document, ingestion requests from the user\n- delivers: wiki pages, instruction updates and research notes to the whole team\n- format: `context/wiki/<subpath>/<slug>.md`, instructions under `context/instructions/`.",
		dod:              "- new/updated documents pass `sdt context lint` (links resolve, no dupes)\n- wiki pages follow the editorial model; `sdt context reindex` picks them up\n- provenance chain is intact (source → transformation → derived document).",
		antiPatterns:     "- inventing conventions instead of using the instruction contracts\n- writing knowledge only in chat and leaving it nowhere\n- duplicating an existing page instead of extending it with a dated entry.",
	},
}

// instrRoleCoreTemplate renders the role-core layer body (9-point structure)
// for one role. Points tied to the project (3 context, 4 conventions) and the
// user (5 preferences) are layer boundaries, resolved by the project and
// preferences scopes respectively.

func instrRoleCoreTemplate(r roleDescriptor) (string, bool) {
	c, ok := roleCoreContents[r.Slug]
	if !ok {
		return "", false
	}
	slug := "`context/roles/" + r.Slug + "`"
	core := "## " + r.Title + " — role profile\n\n"
	core += "Read `context/roles/shared.md` first: this profile states only what is\nspecific to the **" + r.Title + "** role and references the shared rules.\n\n"
	core += "### 1. Mission\n\n" + c.mission + "\n\n"
	core += "### 2. Responsibilities and non-responsibilities\n\n**Owns**\n\n" + c.responsibilities + "\n\n**Does not**\n\n" + c.nonResp + "\n\n"
	core += "### 3. Role-relevant project context\n\nThe concrete stack, layout and commands for this role live in the **project\nlayer** of " + slug + "/project (see the `project` section below); the core\nadds nothing project-specific, so a repo change never rewrites this layer.\n\n"
	core += "### 4. Conventions and constraints\n\nFollow the project layer's real paths and commands plus the shared rules. Role-owning\nconvention areas are listed in `shared.md`'s register table (column\n`owned`). Do not restate or duplicate them here.\n\n"
	core += "### 5. User preferences\n\nLive in the **preferences** section below, user-owned: `--force` never\noverwrites them. The core does not inline user choices.\n\n"
	core += "### 6. Working mode\n\n" + c.workingMode + "\n\n"
	core += "### 7. Handoff (capability contract)\n\n" + c.handoff + "\n\n"
	core += "### 8. Definition of done\n\n" + c.dod + "\n\n"
	core += "### 9. Anti-patterns\n\n" + c.antiPatterns + "\n"
	return core, true
}

// roleSharedFrontmatter is the frontmatter for role profile files.
func roleFrontmatter(r roleDescriptor, project string, now time.Time) string {
	return strings.Join([]string{
		"---",
		"kind: role",
		"title: \"" + r.Title + "\"",
		"slug: " + r.Slug,
		"summary: \"Generated role profile for " + r.Slug + " (closed register); shared rules in context/roles/shared.md.\"",
		"created: \"" + now.UTC().Format(time.RFC3339) + "\"",
		"project: " + project,
		"---",
	}, "\n") + "\n"
}

// rolePreferencesStub is the initial user-owned preferences body. --force never
// touches this scope.
func rolePreferencesStub() string {
	return "## User preferences\n\n(_user-owned_: `sdt agent roles init --force` never overwrites this block; the\nuser edits it directly.)\n"
}

// renderRoleProfile renders the full generated profile document for one role in
// its three layered marker scopes, using the same marker-merge helpers as the
// writer so the drift guard and the on-disk result never diverge. Facts are
// derived from repo evidence on every render, so the profile stays in sync with
// the repository.
func renderRoleProfile(r roleDescriptor, project string, now time.Time) (string, bool) {
	return renderRoleProfileFacts(r, project, now, deriveRoleProjectFacts())
}

// renderRoleProfileFacts is the facts-aware render used by the writer so a
// single derive pass (plus optional user answers) feeds every profile.
func renderRoleProfileFacts(r roleDescriptor, project string, now time.Time, facts roleProjectFacts) (string, bool) {
	core, ok := instrRoleCoreTemplate(r)
	if !ok {
		return "", false
	}
	content := roleFrontmatter(r, project, now)
	content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerCore), core, true)
	content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerProject), roleProjectLayer(r, facts), true)
	content = agentAppendIfMissing(content, roleSectionName(r.Slug, roleLayerPreferences), rolePreferencesStub())
	return content, true
}

// roleProfileBody is the render entry used by tests and the generated-file set.
func roleProfileBody(r roleDescriptor, project string, now time.Time) (string, bool) {
	return renderRoleProfile(r, project, now)
}

// ── generation into context/roles/ ─────────────────────────────────────────────

// agentWriteRoleFile writes or refreshes one role profile. It is non-destructive
// by default (an existing profile is preserved unless force is set). With force
// the generated `core` and `project` layers are refreshed in place, while the
// user-owned `preferences` scope is never rewritten: a missing preferences scope
// is appended, a present one is left untouched. Frontmatter and any user content
// outside the markers survive.

func agentWriteRoleFile(path string, r roleDescriptor, project string, now time.Time, force bool, facts roleProjectFacts) FileResult {
	res := FileResult{Path: path}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir, user-chosen output
	if err != nil && !os.IsNotExist(err) {
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	exists := err == nil
	if exists && !force {
		res.Status = statusSkipped
		res.Reason = "file already exists (use --force to overwrite)"
		return res
	}

	var content string
	if exists {
		core, ok := instrRoleCoreTemplate(r)
		if !ok {
			res.Status = statusError
			res.Reason = "no authored core template for role " + r.Slug
			return res
		}
		content = string(data)
		content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerCore), core, true)
		content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerProject), roleProjectLayer(r, facts), true)
		content = agentAppendIfMissing(content, roleSectionName(r.Slug, roleLayerPreferences), rolePreferencesStub())
	} else {
		rendered, ok := renderRoleProfileFacts(r, project, now, facts)
		if !ok {
			res.Status = statusError
			res.Reason = "no authored core template for role " + r.Slug
			return res
		}
		content = rendered
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G703 G306 -- user-chosen output
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if exists {
		res.Status = statusUpdated
	} else {
		res.Status = statusCreated
	}
	return res
}

// writeRoleFiles generates the shared rules file and every role profile under
// context/roles/. The shared file is single-scope (agentWriteGeneratedFile); the
// profiles use the three-layer writer that preserves user preferences.

func writeRoleFiles(project string, force bool, facts roleProjectFacts) []FileResult {
	now := time.Now()
	var results []FileResult
	sharedPath := filepath.Join(sdtRolesDir, sdtRolesSharedFile)
	results = append(results, agentWriteGeneratedFile(sharedPath, agentGeneratedMarkerName(filepath.Base(sdtRolesDir), sdtRolesSharedFile), instrRoleSharedTemplate(), force))
	for _, r := range roleRegister {
		path := filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt)
		results = append(results, agentWriteRoleFile(path, r, project, now, force, facts))
	}
	return results
}

// ── generated file set ─────────────────────────────────────────────────────────

// roleProfileFiles returns the generated files under context/roles/: the shared
// rules file plus one profile per register slug, in register order. Used by the
// init phase and by the drift guard.

func roleProfileFiles(project string, now time.Time) []instructionFile {
	files := []instructionFile{{name: filepath.Base(sdtRolesSharedFile), body: instrRoleSharedTemplate()}}
	for _, r := range roleRegister {
		if body, ok := roleProfileBody(r, project, now); ok {
			files = append(files, instructionFile{name: r.Slug + sdtMarkdownExt, body: body})
		}
	}
	return files
}
