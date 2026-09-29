# Lifecycle-edge fixture

The resolved lifecycle relations of the documents in this directory, in one place
both implementations must agree on.

- `context/` is a miniature corpus: the `analysis/`, `plan/` and `tasks/`
  directories the resolver reads, with real frontmatter (`uid`, `analysis_id`,
  `plan_id`, `sources`, `links`).
- `expected-edges.json` is the contract: for every document, the single parent
  the typed relation resolves to (`""` when there is none) and the children that
  name it. `""` is the answer for an unresolvable uid, a `links`-only citation and
  a document outside the chain — the corpus is not repaired by reading it.

Two tests assert this file, and that is the point of it:

- `cli/cmd/context_cascade_test.go` runs the Go resolver (`internal/ctxrel`) over
  this directory;
- `web/src/lib/edges.test.ts` reads the same directory and the same JSON and
  asserts the same edges, the way the viewer payload presents them
  (`analysis` on a plan, `plans` on an analysis, `plan` on a task file).

The cases the fixture deliberately covers: the ordinary chain; a plan that cites a
second analysis in `sources`; a task file that cites another plan in `sources`; a
`links`-only citation; an unresolvable parent uid; a document with no parent.

A `sources` or `links` citation is **never** an edge — before the typed relations
were made authoritative, six implementations each re-derived the edge from those
lists and disagreed.
