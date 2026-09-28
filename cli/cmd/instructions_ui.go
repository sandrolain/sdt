package cmd

import "github.com/sandrolain/sdt/internal/templates"

var (
	instrUITemplate            = templates.Must("instructions/ui.md.tmpl", nil, nil)
	instrUITokensTemplate      = templates.Must("instructions/ui-tokens.md.tmpl", nil, nil)
	instrUIComponentsTemplate  = templates.Must("instructions/ui-components.md.tmpl", nil, nil)
	instrUIA11yTemplate        = templates.Must("instructions/ui-accessibility.md.tmpl", nil, nil)
	instrUITasteTemplate       = templates.Must("instructions/ui-taste.md.tmpl", nil, nil)
	instrUIAdaptersTemplate    = templates.Must("instructions/ui-adapters.md.tmpl", nil, nil)
	instrUIPerformanceTemplate = templates.Must("instructions/ui-performance.md.tmpl", nil, nil)
	instrUILayoutTemplate      = templates.Must("instructions/ui-layout.md.tmpl", nil, nil)
)
