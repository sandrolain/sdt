package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/todo"
)

// ── context todo (idea inbox) ─────────────────────────────────────────────────
//
// context/todo.yaml is CLI-owned: these verbs are the only writer. The register
// holds short, annotate-only ideas for future analyses; every mutation also
// regenerates the human-readable projection context/todo.md. The register only
// annotates — it never starts an analysis, a plan or any operation.

// ctxTodoProjectionPath is the generated Markdown projection of the register. It
// is CLI-written, excluded from the corpus and never hand-edited.
const ctxTodoProjectionPath = "context/todo.md"

var contextTodoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage the idea inbox (short annotate-only TODOs)",
	Long: `Manage context/todo.yaml: short, annotate-only ideas for future analyses.

  sdt context todo add "<text>" [--id <slug>] [--source <ref>]
  sdt context todo list
  sdt context todo done <id>
  sdt context todo remove <id>

Adding only annotates: it never starts an analysis, a plan or any operation.
Every mutation regenerates the human-readable projection context/todo.md.`,
}

var contextTodoAddCmd = &cobra.Command{
	Use:   "add <text>",
	Short: "Add a short idea",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		reg := loadTodoRegister(cmd)
		text := strings.TrimSpace(args[0])
		id := reg.IDFor(text, getStringFlag(cmd, "id", false))
		reg.Add(todo.Item{ID: id, Text: text, Created: todo.Today(), Source: getStringFlag(cmd, "source", false)})
		saveTodoRegister(cmd, reg)
		outputTodoResult(cmd, "added", id)
	},
}

var contextTodoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the inbox items",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		reg := loadTodoRegister(cmd)
		if outputTodoStructured(cmd, reg.Items) {
			return
		}
		var b strings.Builder
		for _, it := range reg.Items {
			mark := "[ ]"
			if it.Done {
				mark = "[x]"
			}
			fmt.Fprintf(&b, "%s\t%s\t%s\n", mark, it.ID, it.Text)
		}
		outputString(cmd, b.String())
	},
}

var contextTodoDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark an idea done (kept in the list)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		reg := loadTodoRegister(cmd)
		id := sanitizeSlug(args[0])
		if !reg.MarkDone(id) {
			exitWithError(cmd, fmt.Errorf("no item with id %q", id))
			return
		}
		saveTodoRegister(cmd, reg)
		outputTodoResult(cmd, "done", id)
	},
}

var contextTodoRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an idea from the inbox",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		reg := loadTodoRegister(cmd)
		id := sanitizeSlug(args[0])
		if !reg.Remove(id) {
			exitWithError(cmd, fmt.Errorf("no item with id %q", id))
			return
		}
		saveTodoRegister(cmd, reg)
		outputTodoResult(cmd, "removed", id)
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

func loadTodoRegister(cmd *cobra.Command) *todo.Register {
	reg, err := todo.Load(".")
	if err != nil {
		exitWithError(cmd, err)
		return &todo.Register{}
	}
	return reg
}

// saveTodoRegister writes the register and regenerates the Markdown projection,
// so context/todo.md never drifts from context/todo.yaml.
func saveTodoRegister(cmd *cobra.Command, reg *todo.Register) {
	exitWithError(cmd, todo.Save(".", reg))
	if err := os.WriteFile(ctxTodoProjectionPath, []byte(reg.Markdown()), 0o644); err != nil { //#nosec G306 -- generated projection
		exitWithError(cmd, err)
	}
}

// todoResult is the serializable projection of a mutation for json/yaml output.
type todoResult struct {
	Action string `json:"action" yaml:"action"`
	ID     string `json:"id" yaml:"id"`
}

func outputTodoResult(cmd *cobra.Command, action, id string) {
	if outputTodoStructured(cmd, todoResult{Action: action, ID: id}) {
		return
	}
	outputString(cmd, action+" "+id+"\n")
}

// outputTodoStructured writes v as json/yaml for the selected --format and
// reports whether it handled the output (false for the default text format).
func outputTodoStructured(cmd *cobra.Command, v any) bool {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(v, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, append(out, '\n'))
		return true
	case fmtYAML:
		out, err := yaml.Marshal(v)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
		return true
	}
	return false
}

func init() {
	contextTodoAddCmd.Flags().String("id", "", "Explicit item id (kebab-case slug; default: derived from the text)")
	contextTodoAddCmd.Flags().String("source", "", "Optional document reference the idea came from")

	contextTodoCmd.AddCommand(contextTodoAddCmd, contextTodoListCmd, contextTodoDoneCmd, contextTodoRemoveCmd)
}
