package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrBrowserTemplate      = templates.Must("instructions/browser.md.tmpl", nil, nil)
	instrBrowserToolsTemplate = templates.Must("instructions/browser-tools.md.tmpl", nil, nil)
)
