package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrDistillationTemplate = templates.Must("instructions/distillation.md.tmpl", nil, nil)
)
