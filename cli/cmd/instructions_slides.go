package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrSlidesTemplate     = templates.Must("instructions/slides.md.tmpl", nil, nil)
	instrSlidesMarpTemplate = templates.Must("instructions/slides-marp.md.tmpl", nil, nil)
)
