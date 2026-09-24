package cmd

const instrGitTemplate = `# Git — commits and branches

` + "`context/instructions/git.md`" + ` is the version-control contract: when to
commit, the message format (Conventional Commits v1.0.0), staging discipline and
branch handling. Read it before staging or committing any change.

## The commit gate (ask before you commit)

- After completing a **task file** (see ` + "`context/instructions/tasks.md`" + `, execution
  workflow), if the task produced tracked changes, **propose a commit message and
  ask the user whether to commit**. Never commit without explicit approval.
- One commit per task, scoped to the task's deliverable: stage only the files that
  task changed; do not fold unrelated changes into the same commit.
- Run the task's verification (build/test/lint, the verify-step) **before**
  proposing the commit — a commit captures a verified state.
- If the user declines, leave the working tree as is and continue; record the
  proposed message in the worklog if useful.
- ` + "`context/`" + ` may be gitignored (the repo tracks code only). The gate covers
  tracked files; context documents are committed only if the project versions
  them.

## Commit messages — Conventional Commits v1.0.0

Spec: <https://www.conventionalcommits.org/en/v1.0.0/>

` + codeFence + `text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
` + codeFence + `

- **type** (required): ` + "`feat`" + ` · ` + "`fix`" + ` · ` + "`docs`" + ` · ` + "`style`" + ` · ` + "`refactor`" + ` · ` + "`perf`" + ` ·
  ` + "`test`" + ` · ` + "`build`" + ` · ` + "`ci`" + ` · ` + "`chore`" + ` · ` + "`revert`" + `.
- **scope** (optional): noun in parentheses naming the affected area, e.g.
  ` + "`feat(search): …`" + `.
- **description**: imperative mood, ≤50 chars, lowercase after ` + "`type:`" + `, no
  trailing period.
- **body**: explain **why** (not what) when the reason is not obvious; wrap at 72.
- **footers**: ` + "`BREAKING CHANGE: <description>`" + ` (or ` + "`!`" + ` after the type/scope)
  for incompatible changes; ` + "`Refs: <task/plan>`" + ` for traceability.
- One logical change per commit; do not mix a refactor with a feature.

Examples:

` + codeFence + `text
feat(search): add filename-boosted ranking
fix(viewer): stop tree panel from expanding
docs(git): document the commit gate
refactor!: drop the legacy memory store
` + codeFence + `

## Branches

- **Default: work on the current branch.** Commit per task; do not create a
  branch unless the plan declares a strategy or the user asks.
- A plan may declare a branch strategy in its Design contract; follow it.
- Naming when a branch is used: ` + "`feat/<slug>`" + `, ` + "`fix/<slug>`" + `,
  ` + "`chore/<slug>`" + `, where ` + "`<slug>`" + ` matches the plan or task slug.
- Never force-push a shared branch; never commit secrets or artifacts already
  ignored by ` + "`.gitignore`" + `.
- Merging or opening a PR is a user decision: propose it, do not act autonomously.

## Close out

- After a commit, record the outcome (message, hash when available) in the
  worklog closeout entry — see ` + "`context/instructions/worklog.md`" + `.
- Reference the task file in a ` + "`Refs:`" + ` footer when the project tracks task
  identities.
`
