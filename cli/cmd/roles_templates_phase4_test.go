package cmd

import (
	"testing"
)

// ── byte-identity goldens (Phase 4) ────────────────────────────────────────────
//
// These goldens freeze the exact byte output of the role .tmpl files for the
// closed register and controlled facts. They are the CI-portable counterpart of
// the local drift guard (TestGeneratedRolesMatchTemplates): a template change
// that alters generated output must update these strings AND the generated
// files under context/roles/ together.

// TestRoleSharedTemplateGolden freezes the shared rules document rendered for
// the closed register: header prose, the register table rows (yesNo/ownedList
// funcs) and the static body sections.
func TestRoleSharedTemplateGolden(t *testing.T) {
	want := `# Shared Role Rules

**Every profile in ` + "`context/roles/`" + ` references this file. Read it before
any role profile.** It is the shared layer: rules common to all roles live here
once, profiles never restate them. Generated from the same closed register the
CLI uses, so the table below cannot drift from ` + "`sdt agent roles show`" + `.

## Role register (closed)

The set is **closed and curated**: adding a role is a deliberate change that
touches the register, the templates and ` + "`roles check`" + `. ` + "`pm`" + ` is the
coordinator: it owns planning/sequencing and handoffs over the seven
execution/review roles.

| slug | title | coordinator | owned (default context/ spaces) |
|---|---|---|---|
| ` + "`pm`" + ` | Project manager | yes | ` + "`context/plan`, `context/tasks`" + ` |
| ` + "`backend`" + ` | Backend engineer | no | — |
| ` + "`frontend`" + ` | Frontend engineer | no | — |
| ` + "`architect`" + ` | Solution architect | no | ` + "`context/architecture`, `context/decisions`, `context/analysis`" + ` |
| ` + "`reviewer`" + ` | Reviewer | no | ` + "`context/proposals`, `context/questions`" + ` |
| ` + "`qa`" + ` | Quality assurance | no | — |
| ` + "`devops`" + ` | DevOps engineer | no | — |
| ` + "`docs`" + ` | Documentation engineer | no | ` + "`context/wiki`, `context/instructions`, `context/research`" + ` |


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
	if got := instrRoleSharedTemplate(); got != want {
		t.Errorf("shared.md.tmpl drifted from golden:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

// TestRoleCoreTemplateGolden freezes the 9-point core body for one register
// role, pinning the H2 intro ({{.Title}}), the layer-boundary references
// ({{.Slug}}) and every static skeleton section around the authored content.
func TestRoleCoreTemplateGolden(t *testing.T) {
	core, ok := instrRoleCoreTemplate(roleDescriptor{Slug: "reviewer", Title: "Reviewer"})
	if !ok {
		t.Fatal("reviewer core template missing")
	}
	want := `## Reviewer — role profile

Read ` + "`context/roles/shared.md`" + ` first: this profile states only what is
specific to the **Reviewer** role and references the shared rules.

### 1. Mission

Provide the independent second pass: verify phases and deliverables against their claims, using evidence, never self-approval.

### 2. Responsibilities and non-responsibilities

**Owns**

- review deliverables (code, docs, phases) against the task-file claims and the project's gates
- run the verify-step and record CONFIRMED / DISPROVED / UNVERIFIED verdicts
- own the review surface: proposals and open questions get closed verdicts.

**Does not**

- does not implement the work it reviews
- does not rubber-stamp; every verdict needs evidence (command output, file:line)
- does not enforce rules that need a runtime SDT cannot provide (advisory-only).

### 3. Role-relevant project context

The concrete stack, layout and commands for this role live in the **project
layer** of ` + "`context/roles/reviewer`" + `/project (see the ` + "`project`" + ` section below); the core
adds nothing project-specific, so a repo change never rewrites this layer.

### 4. Conventions and constraints

Follow the project layer's real paths and commands plus the shared rules. Role-owning
convention areas are listed in ` + "`shared.md`" + `'s register table (column
` + "`owned`" + `). Do not restate or duplicate them here.

### 5. User preferences

Live in the **preferences** section below, user-owned: ` + "`--force`" + ` never
overwrites them. The core does not inline user choices.

### 6. Working mode

- independent pass: the author of the work does not self-approve
- degrade gracefully: CRITICAL before WARNING before SUGGESTION
- reproduce claims with the real commands before confirming them.

### 7. Handoff (capability contract)

- receives: completed code/phases from executor roles, proposals from the proposer
- delivers: review verdicts to pm and the authors
- format: task-file ` + "`## Review`" + ` blocks and proposal verdicts (accepted/rejected).

### 8. Definition of done

- each finding ends in a closed verdict with evidence
- review gate passes (` + "`sdt agent gate`" + ` when a hard gate is wanted)
- findings are actionable: hint + remediation next action.

### 9. Anti-patterns

- approving without running the commands (claims marked passed must be run)
- reviewing one's own work
- flagging without a remediation hint.
`
	if core != want {
		t.Errorf("reviewer core drifted from golden:\ngot:\n%q\nwant:\n%q", core, want)
	}
}

// TestRoleCoreTemplateMissingSlug checks the (string, bool) contract: an
// unknown slug reports ok=false and renders nothing.
func TestRoleCoreTemplateMissingSlug(t *testing.T) {
	if body, ok := instrRoleCoreTemplate(roleDescriptor{Slug: "nonexistent", Title: "Nope"}); ok || body != "" {
		t.Errorf("unknown slug must report ok=false and empty body, got body=%q ok=%v", body, ok)
	}
}

// TestRoleProjectLayerGolden freezes the project-layer body for controlled
// facts: the fully-derived case (every optional section present, tagged (repo)
// and (user)) and the assumption-footer case (nothing derivable, tagged
// (assumption)).
func TestRoleProjectLayerGolden(t *testing.T) {
	r := roleDescriptor{Slug: "reviewer", Title: "Reviewer"}

	full := roleProjectFacts{
		Stack:       "Go 1.27 · pure-Go",
		Build:       "task build",
		Test:        "task test",
		Lint:        "golangci-lint run",
		Layout:      []string{"cli", "internal", "web"},
		OwnedPaths:  map[string][]string{"reviewer": {"context/proposals"}},
		Conventions: []string{"no CGO", "80% coverage"},
		UserAnswers: []string{"Independent review pairing → backend+frontend cross-review"},
	}
	wantFull := `## Project layer (` + "`context/roles/reviewer`" + `)

Repo-derived facts for this role. Lines are tagged by source:
` + "`(repo)`" + ` from repo evidence, ` + "`(user)`" + ` answered by the user,
` + "`(assumption)`" + ` not derivable — resolve before treating it as a rule.
Regenerated by ` + "`sdt agent roles init --force`" + `; do not hand-edit here.

- Stack: Go 1.27 · pure-Go (repo)
- Build: task build (repo)
- Test: task test (repo)
- Lint: golangci-lint run (repo)
- Repo layout: ` + "`cli`, `internal`, `web`" + ` (repo)
- Owned repo paths: ` + "`context/proposals`" + ` (repo)
- Conventions:
  - no CGO (repo)
  - 80% coverage (repo)
- Independent review pairing → backend+frontend cross-review (user)

Project-specific conventions also live in ` + "`context/instructions/project.md`" + `
and the AGENTS.md project block — reference them, never restate them.
`
	if got := roleProjectLayer(r, full); got != wantFull {
		t.Errorf("full project layer drifted from golden:\ngot:\n%q\nwant:\n%q", got, wantFull)
	}

	sparse := roleProjectFacts{
		Assumptions: []string{
			"deploy target not derivable (add a Conventions section or answer the questionnaire)",
		},
	}
	wantSparse := `## Project layer (` + "`context/roles/reviewer`" + `)

Repo-derived facts for this role. Lines are tagged by source:
` + "`(repo)`" + ` from repo evidence, ` + "`(user)`" + ` answered by the user,
` + "`(assumption)`" + ` not derivable — resolve before treating it as a rule.
Regenerated by ` + "`sdt agent roles init --force`" + `; do not hand-edit here.

- Stack: not derivable (assumption)
- Build: not derivable (assumption)
- Test: not derivable (assumption)
- Lint: not derivable (assumption)

Project-specific conventions also live in ` + "`context/instructions/project.md`" + `
and the AGENTS.md project block — reference them, never restate them.

### Open assumptions

Not derivable from repo evidence and not answered by the user. Resolve with
evidence or the ≤3-question questionnaire; never left as silent rules.

- deploy target not derivable (add a Conventions section or answer the questionnaire) (assumption)
`
	if got := roleProjectLayer(r, sparse); got != wantSparse {
		t.Errorf("sparse project layer drifted from golden:\ngot:\n%q\nwant:\n%q", got, wantSparse)
	}
}
