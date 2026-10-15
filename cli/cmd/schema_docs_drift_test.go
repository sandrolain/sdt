package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// The two discovery surfaces an agent reads instead of scraping --help are
// `sdt schema` and the generated per-command docs. They used to enumerate the
// command tree through two separate walks with different filters, which is how
// they drifted apart without anything failing: the schema carried `help` and the
// docs did not. They now share one walk, and this file is the guard that keeps
// it that way.

// schemaCommandPaths returns the command paths `sdt schema` reports, taken from
// the same builder the command uses rather than from a hand-written list.
func schemaCommandPaths() []string {
	var schemas []CommandSchema
	collectSchemas(rootCmd, &schemas)
	out := make([]string, 0, len(schemas))
	for _, s := range schemas {
		out = append(out, s.Command)
	}
	sort.Strings(out)
	return out
}

// docFilePaths returns the command paths the generated docs cover, derived from
// the file names the generator produces.
func docFilePaths(t *testing.T) []string {
	t.Helper()
	leaves := allDocsLeaves()
	out := make([]string, 0, len(leaves))
	for _, l := range leaves {
		out = append(out, l.path)
	}
	sort.Strings(out)
	return out
}

// TestSchemaAndDocsEnumerateTheSameCommands is the drift guard. It fails the
// moment the two surfaces disagree, whichever side moved.
func TestSchemaAndDocsEnumerateTheSameCommands(t *testing.T) {
	schema := schemaCommandPaths()
	docs := docFilePaths(t)
	if len(schema) == 0 || len(docs) == 0 {
		t.Fatal("one of the surfaces enumerated nothing")
	}
	if strings.Join(schema, "\n") != strings.Join(docs, "\n") {
		missingDoc := diffStrings(schema, docs)
		missingCmd := diffStrings(docs, schema)
		t.Errorf("schema and docs disagree:\n  reported by the schema with no doc: %v\n  documented with no schema entry: %v",
			missingDoc, missingCmd)
	}
}

// TestCobraBuiltinsAreExcludedFromBothSurfaces pins the exclusion as a decision
// rather than an accident of one walk: a generated page for cobra's own help
// would be a page about cobra, and a schema entry for it reports nothing.
func TestCobraBuiltinsAreExcludedFromBothSurfaces(t *testing.T) {
	for _, name := range []string{"help", "completion"} {
		for _, p := range schemaCommandPaths() {
			if p == name {
				t.Errorf("%s is reported by the schema; cobra's built-ins are excluded from both surfaces", name)
			}
		}
		for _, p := range docFilePaths(t) {
			if p == name {
				t.Errorf("%s is documented; cobra's built-ins are excluded from both surfaces", name)
			}
		}
	}
}

// TestGeneratedDocsCoverTheSchemaOnDisk binds the guard to the committed
// directory as well as to the generator: a walk that agrees with itself still
// leaves a stale file behind if nobody regenerates.
func TestGeneratedDocsCoverTheSchemaOnDisk(t *testing.T) {
	root := projectRoot(t)
	dir := filepath.Join(root, "context", "sdtdocs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the generated docs are not present: %v", err)
	}
	onDisk := map[string]struct{}{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), sdtMarkdownExt) {
			continue
		}
		if e.Name() == readmeFile {
			continue // the index, which is not a command
		}
		onDisk[strings.TrimSuffix(e.Name(), sdtMarkdownExt)] = struct{}{}
	}
	if len(onDisk) == 0 {
		t.Fatal("no generated docs found")
	}
	var missing []string
	for _, p := range docFilePaths(t) {
		file := strings.TrimSuffix(docFileName(p), sdtMarkdownExt)
		if _, ok := onDisk[file]; !ok {
			missing = append(missing, docFileName(p))
		}
		delete(onDisk, file)
	}
	if len(missing) > 0 {
		t.Errorf("%d documented command(s) have no file on disk (run `sdt context docs`): %v", len(missing), missing)
	}
	if len(onDisk) > 0 {
		stale := make([]string, 0, len(onDisk))
		for name := range onDisk {
			stale = append(stale, name+sdtMarkdownExt)
		}
		sort.Strings(stale)
		t.Errorf("%d file(s) on disk document no command (run `sdt context docs --clean`): %v", len(stale), stale)
	}
}

// TestDocFileNamesAreUnique guards the flattening from silently overwriting a
// page: two command paths that normalise to one file name would lose one.
func TestDocFileNamesAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, p := range docFilePaths(t) {
		f := docFileName(p)
		if prev, dup := seen[f]; dup {
			t.Errorf("commands %q and %q share the file name %q", prev, p, f)
		}
		seen[f] = p
	}
	if len(seen) == 0 {
		t.Fatal("no doc file names derived")
	}
}

// TestDocFileNameKeepsTheCommandPathIntact is a light shape check on the
// flattening rule the uniqueness above depends on.
func TestDocFileNameKeepsTheCommandPathIntact(t *testing.T) {
	if got := docFileName("context task add"); got != "context-task-add.md" {
		t.Errorf("docFileName = %q, want context-task-add.md", got)
	}
	if regexp.MustCompile(`[^a-z0-9.-]`).MatchString(docFileName("a b c")) {
		t.Error("a doc file name carries a character a filesystem may object to")
	}
}

// diffStrings returns the entries of a that are absent from b.
func diffStrings(a, b []string) []string {
	in := make(map[string]struct{}, len(b))
	for _, s := range b {
		in[s] = struct{}{}
	}
	var out []string
	for _, s := range a {
		if _, ok := in[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

// TestEveryRunnableCommandIsInTheSchema is the property the shared walk must
// satisfy, asserted as a property rather than as a list of today's commands: a
// list freezes the size of the CLI and passes on the next omission.
//
// A command with a `Run` can be invoked on its own — `sdt b64 <text>` and
// `sdt doc2md <file>... [flags]` are the primary forms of their commands — so the
// surface the local-reads rule sends an agent to must describe it. The walk this
// guards once required an empty sub-command list, which dropped every runnable
// group: nine at the root and the nested `context status`.
func TestEveryRunnableCommandIsInTheSchema(t *testing.T) {
	schema := map[string]bool{}
	for _, p := range schemaCommandPaths() {
		schema[p] = true
	}
	if len(schema) == 0 {
		t.Fatal("the schema enumerated nothing — the assertion below would prove nothing")
	}

	var missing []string
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		if c.Hidden {
			return
		}
		if _, builtin := cobraBuiltins[c.Name()]; builtin {
			return
		}
		full := c.Name()
		if path != "" {
			full = path + " " + c.Name()
		}
		if c.Run != nil && !schema[full] {
			missing = append(missing, full)
		}
		for _, sub := range c.Commands() {
			walk(sub, full)
		}
	}
	for _, sub := range rootCmd.Commands() {
		walk(sub, "")
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("runnable command(s) absent from `sdt schema`: %v — a command with a Run is invocable and must be described", missing)
	}
}
