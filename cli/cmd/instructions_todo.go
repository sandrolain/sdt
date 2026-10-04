package cmd

import (
	"github.com/sandrolain/sdt/internal/templates"
	"github.com/sandrolain/sdt/internal/todo"
)

// ctxTodoFilePath is the on-disk idea-inbox register. It is seeded at init and
// mutated only through `sdt context todo`.
const ctxTodoFilePath = "context/todo.yaml"

// todoRegisterTemplate seeds context/todo.yaml: the idea inbox of short,
// annotate-only objectives and ideas. Generated at init, CLI-owned and
// user-readable thereafter.

var todoRegisterTemplate = templates.Must("workspace/todo.yaml.tmpl", nil, nil)

// validateTodoRegister reads and validates the idea-inbox register for lint. A
// missing file is not a finding; a malformed or invalid one returns an error the
// caller reports as an advisory WARNING on the register path.

func validateTodoRegister() error {
	_, err := todo.Load(".")
	return err
}
