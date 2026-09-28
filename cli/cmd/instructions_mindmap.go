package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrMindmapTemplate        = templates.Must("instructions/mindmap.md.tmpl", nil, nil)
	instrMindmapMarkmapTemplate = templates.Must("instructions/mindmap-markmap.md.tmpl", nil, nil)
)
