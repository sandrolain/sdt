package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrWritingTemplate = templates.Must("instructions/writing.md.tmpl", nil, nil)
)
