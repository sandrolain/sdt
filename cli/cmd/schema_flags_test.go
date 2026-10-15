package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// flagSchemaTree builds a small command tree exercising the three shapes the
// schema builder has to get right: a command declaring on Flags(), a child
// inheriting the parent's persistent flag, and a child that redeclares a name
// the parent already declared.
func flagSchemaTree() (*cobra.Command, *cobra.Command, *cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("log-format", "text", "Log format")
	root.PersistentFlags().Bool("quiet", false, "Suppress output")

	inherits := &cobra.Command{Use: "inherits", Run: func(*cobra.Command, []string) {}}
	inherits.Flags().String("dir", "", "Directory to search")

	shadows := &cobra.Command{Use: "shadows", Run: func(*cobra.Command, []string) {}}
	shadows.Flags().String("log-format", "json", "A local log format")

	bare := &cobra.Command{Use: "bare", Run: func(*cobra.Command, []string) {}}

	root.AddCommand(inherits, shadows, bare)
	return root, inherits, shadows, bare
}

// TestFlagSchemaReportsADeclaredPersistentFlag is the defect this phase exists
// to fix: a command whose whole option set lives on PersistentFlags() reported a
// bare empty object, telling an agent the verb takes no options.
func TestFlagSchemaReportsADeclaredPersistentFlag(t *testing.T) {
	root, _, _, _ := flagSchemaTree()
	s := buildFlagSchema(root)
	if s.Properties == nil {
		t.Fatal("a command with persistent flags reported an empty flags object")
	}
	if _, ok := s.Properties["log-format"]; !ok {
		t.Errorf("a flag declared on PersistentFlags() is missing: %v", keysOf(s.Properties))
	}
}

// TestFlagSchemaMergesLocalAndInherited pins both halves of the merged set: a
// command's own flag and the globals it can actually be passed.
func TestFlagSchemaMergesLocalAndInherited(t *testing.T) {
	_, inherits, _, _ := flagSchemaTree()
	s := buildFlagSchema(inherits)
	for _, want := range []string{"dir", "log-format", "quiet"} {
		if _, ok := s.Properties[want]; !ok {
			t.Errorf("merged flag set is missing %q: %v", want, keysOf(s.Properties))
		}
	}
	// The schema reports what a caller can pass, so the count is the union and
	// not the sum: three names, not four.
	if len(s.Properties) != 3 {
		t.Errorf("flag count = %d, want 3 (the union): %v", len(s.Properties), keysOf(s.Properties))
	}
}

// TestFlagSchemaShadowedGlobalIsReportedOnce is the analysis' open risk: a
// command redeclaring a name the parent already declared must produce one entry,
// from the local declaration, with map iteration order deciding nothing.
func TestFlagSchemaShadowedGlobalIsReportedOnce(t *testing.T) {
	_, _, shadows, _ := flagSchemaTree()
	s := buildFlagSchema(shadows)
	// A map cannot hold a duplicate key, so the risk is not a doubled entry but
	// which declaration wins. The local one must, because that is the flag the
	// command parses.
	got, ok := s.Properties["log-format"]
	if !ok {
		t.Fatalf("the shadowing command lost the flag: %v", keysOf(s.Properties))
	}
	if got.Default != "json" {
		t.Errorf("log-format default = %v, want the local declaration (json), not the inherited one", got.Default)
	}
	// Run it repeatedly: iteration order must not change the answer.
	for i := 0; i < 50; i++ {
		again := buildFlagSchema(shadows)
		if again.Properties["log-format"].Default != got.Default {
			t.Fatalf("iteration %d gave a different entry: %v", i, again.Properties["log-format"].Default)
		}
		if len(again.Properties) != len(s.Properties) {
			t.Fatalf("iteration %d gave %d flags, want %d", i, len(again.Properties), len(s.Properties))
		}
	}
}

// TestFlagSchemaEmptyOnlyWhenThereAreNoFlags keeps the empty-object fallback
// honest: it is legitimate for a command with no flags and nothing to inherit,
// and must not be a symptom of reading the wrong flag set.
//
// In this CLI the fallback is effectively unreachable: every command inherits
// the root's globals, which is why the corpus-wide count below is zero and not
// "only those that declare something".
func TestFlagSchemaEmptyOnlyWhenThereAreNoFlags(t *testing.T) {
	barren := &cobra.Command{Use: "barren", Run: func(*cobra.Command, []string) {}}
	s := buildFlagSchema(barren)
	if s.Properties != nil {
		t.Errorf("a command with no flags reported %v", keysOf(s.Properties))
	}
	if s.Type != "object" {
		t.Errorf("fallback type = %q, want object", s.Type)
	}

	// A child of a command that declares persistent flags is never flag-less:
	// what it can be passed is the union, not its own declarations.
	_, _, _, child := flagSchemaTree()
	if got := buildFlagSchema(child); len(got.Properties) == 0 {
		t.Error("a child inheriting persistent flags reported an empty set")
	}
}

// TestEveryShippedCommandReportsItsFlags is the corpus-wide claim, read from the
// real tree rather than a fixture: measured before the fix, 83 of 173 commands
// emitted an empty flags object.
func TestEveryShippedCommandReportsItsFlags(t *testing.T) {
	var schemas []CommandSchema
	collectSchemas(rootCmd, &schemas)
	if len(schemas) == 0 {
		t.Fatal("no schemas built")
	}
	var empty []string
	for _, s := range schemas {
		if s.Flags == nil || len(s.Flags.Properties) == 0 {
			empty = append(empty, s.Command)
		}
	}
	if len(empty) > 0 {
		t.Errorf("%d command(s) still report no flags, so the merged set is still not being read: %v",
			len(empty), empty)
	}
	// Every command inherits the global flags, so each entry must carry them.
	for _, s := range schemas {
		if _, ok := s.Flags.Properties[flagNameFormat]; !ok {
			t.Errorf("%s does not report the global --%s it inherits", s.Command, "format")
		}
	}
}

// flagNameFormat is the global the schema must report on every command, since
// every command inherits it.
const flagNameFormat = "format"

func keysOf(m map[string]*JSONSchema) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
