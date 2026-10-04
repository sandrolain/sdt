package cmd

import (
	"github.com/sandrolain/sdt/internal/memo"
	"github.com/sandrolain/sdt/internal/templates"
)

// ctxMemoFilePath is the on-disk planned-operation register. It is seeded at
// init and mutated only through `sdt context memo`.
const ctxMemoFilePath = "context/memo.yaml"

// memoRegisterTemplate seeds context/memo.yaml: the planned-operation register
// (recurring rules and one-off memos with their last-run state). Generated at
// init, CLI-owned and user-readable thereafter.

var memoRegisterTemplate = templates.Must("workspace/memo.yaml.tmpl", nil, nil)

// validateMemoRegister reads and validates the planned-operation register for
// lint. A missing file is not a finding; a malformed or invalid one returns an
// error the caller reports as an advisory WARNING on the register path.

func validateMemoRegister() error {
	_, err := memo.Load(".")
	return err
}
