package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrVectorTemplate    = templates.Must("instructions/vector.md.tmpl", nil, nil)
	instrVectorSvgTemplate = templates.Must("instructions/vector-svg.md.tmpl", nil, nil)
)
