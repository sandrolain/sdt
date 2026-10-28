package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/sandrolain/sdt/internal/templates"
)

// The distribution gate, second half: the surfaces that live in this package.
//
// `internal/templates` owns the pattern table and checks the templates, the
// AGENTS.md block and the roles core. The trigger examples, the command help and
// the structural "cited but not generated" rule are here, because only this
// package knows the rendered set.

// leakSurfacesInCmd returns every generated surface owned by this package.
func leakSurfacesInCmd() map[string]string {
	surfaces := map[string]string{}
	var examples strings.Builder
	for _, s := range agentCommandStubs {
		for _, e := range s.examples {
			examples.WriteString(e + "\n")
		}
	}
	surfaces["agentCommandStubs.examples"] = examples.String()

	var help strings.Builder
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		help.WriteString(c.Short + "\n" + c.Long + "\n" + c.Example + "\n")
		c.Flags().VisitAll(func(f *pflag.Flag) {
			help.WriteString(f.Usage + "\n")
		})
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(rootCmd)
	surfaces["rootCmd help"] = help.String()
	return surfaces
}

// knownCmdLeaks is the audit's remaining backlog in this package (phase 4 of the
// plan empties it), keyed by `surface|pattern` with the expected hit count.
var knownCmdLeaks = map[string]int{}

// TestCommandSurfacesLeaks runs the shared table over the examples and the help
// text. A command's help is read by the recipient's agent exactly like a module.
func TestCommandSurfacesLeaks(t *testing.T) {
	for name, body := range leakSurfacesInCmd() {
		got := map[string]int{}
		for _, hit := range templates.ScanLeaks(body) {
			got[hit.Pattern]++
			if _, allowed := knownCmdLeaks[name+"|"+hit.Pattern]; !allowed {
				t.Errorf("%s: [%s] %q — %s", name, hit.Pattern, hit.Text, hit.Reason)
			}
		}
		for key, want := range knownCmdLeaks {
			if !strings.HasPrefix(key, name+"|") {
				continue
			}
			pattern := strings.TrimPrefix(key, name+"|")
			if got[pattern] != want {
				t.Errorf("allowlist stale: %s declared %d, found %d", key, want, got[pattern])
			}
		}
	}
}

// TestGateCommandNamesNoToolchain pins the L2/LP3 invariant: the shipped gate
// help must speak about the project's declaration, never about a Go ladder.
func TestGateCommandNamesNoToolchain(t *testing.T) {
	help := agentGateCmd.Short + "\n" + agentGateCmd.Long
	if !strings.Contains(help, "gate:") {
		t.Errorf("the gate help must name the `gate:` declaration:\n%s", help)
	}
	for _, forbidden := range []string{"golangci", "go build", "go vet", "go test", "Build->Vet"} {
		if strings.Contains(help, forbidden) {
			t.Errorf("the gate help names %q, a toolchain the recipient may not have:\n%s", forbidden, help)
		}
	}
}

// citedPathRe matches a markdown path the set points the reader at.
// A cited document is either under a kind directory or at the corpus root
// (`context/index.md`, `context/essentials.md`): the first pattern alone let a
// top-level generated file pass unguarded.
var citedPathRe = regexp.MustCompile("`(context/(?:[a-z]+/)?[A-Za-z0-9_.<>/-]+\\.md)`")

// TestCitedContextDocsAreGenerated fails on a generated surface that cites a
// `context/...md` file no command creates. This is the L1 class: twenty-two
// instructions treated a document as authoritative that a fresh project does not
// have, and no test noticed.
func TestCitedContextDocsAreGenerated(t *testing.T) {
	produced := generatedContextDocs()

	var bodies []string
	for _, f := range instructionFiles("p", "g") {
		bodies = append(bodies, f.body)
	}
	for _, f := range commandFiles("p", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		bodies = append(bodies, f.body)
	}
	for _, f := range roleProfileFiles("p", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		bodies = append(bodies, f.body)
	}

	seen := map[string]bool{}
	for _, body := range bodies {
		for _, m := range citedPathRe.FindAllStringSubmatch(body, -1) {
			path := m[1]
			if strings.ContainsAny(path, "<>") || seen[path] {
				continue // a placeholder names the shape, not a file
			}
			seen[path] = true
			if !citedPathUnproduced(path, produced) {
				continue
			}
			t.Errorf("cited but not generated: %s — no command writes it and it is not a project document directory", path)
		}
	}
}

// citedPathUnproduced reports a cited document that neither a command writes nor
// a project directory holds.
func citedPathUnproduced(path string, produced map[string]bool) bool {
	if produced[path] {
		return false
	}
	dir := path[:strings.LastIndexByte(path, '/')]
	if kindDirs[dir] {
		return false
	}
	// A subpath kind keeps its documents in nested directories, so a cited path
	// belongs to the kind when its root is a registered document directory.
	for root := range kindDirs {
		if strings.HasPrefix(path, root+"/") {
			return false
		}
	}
	return true
}

// generatedContextDocs is the set of context paths `agent init` writes, plus the
// files the CLI itself maintains.
func generatedContextDocs() map[string]bool {
	out := map[string]bool{
		"context/index.md":          true,
		sdtEssentialsFile:           true,
		ctxTodoProjectionPath:       true,
		"context/commands/index.md": true,
		sdtScriptsIndex:             true,
		sdtDocsReadme:               true,
	}
	for _, f := range instructionFiles("", "") {
		out["context/instructions/"+f.name] = true
	}
	for _, f := range commandFiles("", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		out["context/commands/"+f.name] = true
	}
	for _, f := range roleProfileFiles("", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		out["context/roles/"+f.name] = true
	}
	return out
}

// kindDirs is the set of directories a project fills with its own documents.
func init() {
	kindDirs = map[string]bool{}
	for _, t := range contextTypeList {
		kindDirs[t.dir] = true
	}
	kindDirs[sdtTmpDir] = true
	kindDirs[sdtTrashDir] = true
	kindDirs[sdtRefsDir] = true
	kindDirs[sdtIngestionDir] = true
}

// kindDirs is populated from the type registry at init.
var kindDirs map[string]bool

// producedExampleRoots are the corpus roots the generated set creates. An
// example may name a path under one of them because the recipient will have it;
// any other root is this repository's own layout.
func producedExampleRoots() map[string]bool {
	roots := map[string]bool{sdtWorkDir: true}
	for d := range kindDirs {
		roots[filepath.Base(d)] = true
	}
	return roots
}

// examplePathToken matches a path-like token in a trigger example.
var examplePathToken = regexp.MustCompile(`[A-Za-z0-9_.<>-]+(?:/[A-Za-z0-9_.<>-]*)+`)

// TestTriggerExamplesNameOnlyProducedPaths is the structural half of L3/L14: the
// pattern table catches a name it already knows, and this catches a path that
// looks real but that nothing in the generated set creates. The single-word gate
// this analysis replaced could not see `web/` in an example, because `web/` is
// also ordinary prose ("web/UI surface"); the rule here is about the root, so
// the prose stays and the repository layout fails.
func TestTriggerExamplesNameOnlyProducedPaths(t *testing.T) {
	roots := producedExampleRoots()

	// The rule has to fail on the shape that produced this analysis, or it is a
	// test that can only agree with whatever the examples say.
	if got := examplesWithForeignPath([]string{">wiki: list the pages under web/"}, roots); len(got) != 1 {
		t.Fatalf("the rule accepts the historical leak: got %v", got)
	}
	if got := examplesWithForeignPath([]string{">plan: analysis/<analysis-file>.md"}, roots); len(got) != 0 {
		t.Fatalf("the rule refuses a produced corpus path: got %v", got)
	}

	for _, stub := range agentCommandStubs {
		for _, problem := range examplesWithForeignPath(stub.examples, roots) {
			t.Errorf("trigger %q: %s", stub.id, problem)
		}
	}
}

// examplesWithForeignPath reports each path-like token whose root no generated
// directory creates. A host (example.com/…) and a placeholder (<file>) are not
// project paths.
func examplesWithForeignPath(examples []string, roots map[string]bool) []string {
	var problems []string
	for _, example := range examples {
		for _, tok := range examplePathToken.FindAllString(example, -1) {
			head := strings.SplitN(tok, "/", 2)[0]
			if strings.Contains(head, ".") || strings.HasPrefix(head, "<") || roots[head] {
				continue
			}
			problems = append(problems, fmt.Sprintf("example %q names %q, a path no generated directory creates", example, tok))
		}
	}
	return problems
}

// TestCitedPathCheckSeesTopLevelFiles is the R1 fixture: the session-start
// digest sits at the corpus root, and the check that guards citations must both
// accept it (a producer exists) and refuse a fabricated root file.
func TestCitedPathCheckSeesTopLevelFiles(t *testing.T) {
	produced := generatedContextDocs()
	if !produced[sdtEssentialsFile] {
		t.Fatalf("%s has no declared producer", sdtEssentialsFile)
	}
	if citedPathUnproduced(sdtEssentialsFile, produced) {
		t.Errorf("the check refuses a generated top-level file: %s", sdtEssentialsFile)
	}
	if !citedPathUnproduced("context/ghost.md", produced) {
		t.Error("the check accepts a fabricated top-level file")
	}
	if !citedPathRe.MatchString("`context/essentials.md`") {
		t.Error("the citation pattern does not see a top-level file")
	}
}
