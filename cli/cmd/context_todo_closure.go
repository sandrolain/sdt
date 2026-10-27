package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sandrolain/sdt/internal/todo"
	"github.com/spf13/cobra"
)

// The todo lifecycle's closing half.
//
// An analysis records the register entries it was opened from (`todos:`), a plan
// created from it inherits them, and closing either one marks them done. The rule is
// the user's (2026-10-27): a todo must not outlive the workflow that answered it, and
// the obligation has to be visible where the work is planned, not only where it ends.

// ctxFrontmatterTodos is the frontmatter key carrying the register ids a document
// answers.
const ctxFrontmatterTodos = "todos"

// todoClosureSection is the plan's own item: one checklist box naming the ids, so a
// plan cannot be delivered without meeting it. It sits under its own heading because a
// checklist item appended to the last phase would read as part of that phase.
func todoClosureSection(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		quoted = append(quoted, "`"+id+"`")
	}
	return "\n## Todo closure\n\n- [ ] Close the todos this work answers: " + strings.Join(quoted, ", ") + ".\n"
}

// validateTodoIDs refuses an id the register does not know: a typo would otherwise
// close nothing and nobody would notice.
func validateTodoIDs(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	reg, err := todo.Load(".")
	if err != nil {
		return err
	}
	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := reg.Find(id); !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unknown todo id(s): %s — see `sdt context todo list`", strings.Join(missing, ", "))
	}
	return nil
}

// todoIDsFromDocument reads the `todos:` list of one document, tolerating a file that
// is gone or has no list: the close of a chain must not fail because one link never
// carried an obligation.
func todoIDsFromDocument(path string) []string {
	data, err := os.ReadFile(path) //#nosec G304 -- a document the caller is closing
	if err != nil {
		return nil
	}
	return parseFrontmatterList(string(data), ctxFrontmatterTodos)
}

// closeTodosFor marks done every register entry named by the documents this close
// completed, and returns the ids that actually changed. It is idempotent: a second run
// finds nothing to change and says nothing.
func closeTodosFor(cmd *cobra.Command, paths []string) []string {
	ids := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, path := range paths {
		for _, id := range todoIDsFromDocument(path) {
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	reg, err := todo.Load(".")
	exitWithError(cmd, err)
	closed := make([]string, 0, len(ids))
	for _, id := range ids {
		i, ok := reg.Find(id)
		if !ok || reg.Items[i].Done {
			continue
		}
		reg.MarkDone(id)
		closed = append(closed, id)
	}
	if len(closed) == 0 {
		return nil
	}
	saveTodoRegister(cmd, reg)
	return closed
}
