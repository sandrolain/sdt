package cmd

import (
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
var citedPathRe = regexp.MustCompile("`(context/[a-z]+/[A-Za-z0-9_.<>/-]+\\.md)`")

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
			if produced[path] {
				continue
			}
			dir := path[:strings.LastIndexByte(path, '/')]
			if kindDirs[dir] {
				continue // a document the project writes, not one SDT generates
			}
			t.Errorf("cited but not generated: %s — no command writes it and it is not a project document directory", path)
		}
	}
}

// generatedContextDocs is the set of context paths `agent init` writes, plus the
// files the CLI itself maintains.
func generatedContextDocs() map[string]bool {
	out := map[string]bool{
		"context/index.md":          true,
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
