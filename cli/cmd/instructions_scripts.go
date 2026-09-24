package cmd

import "github.com/sandrolain/sdt/internal/templates"

var instrScriptsTemplate = templates.Must("instructions/scripts.md.tmpl", nil, nil)

var scriptsIndexTemplate = templates.Must("workspace/scripts-index.md.tmpl", nil, nil)
