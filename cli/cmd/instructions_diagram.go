package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrDiagramTemplate        = templates.Must("instructions/diagram.md.tmpl", nil, nil)
	instrDiagramMermaidTemplate = templates.Must("instructions/diagram-mermaid.md.tmpl", nil, nil)
)
