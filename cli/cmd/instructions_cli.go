package cmd

import "github.com/sandrolain/sdt/internal/templates"

// sdtcliCatalogData carries the registry-driven type lists rendered into the
// Agent tooling catalog of cli.md; they derive from the document-type registry
// so the help text never drifts from the surfaces it documents.

type sdtcliCatalogData struct {
	NewTypes  string
	PathTypes string
	ListTypes string
}

// sdtcliCatalogDataFromRegistry resolves the registry-driven type lists for
// the cli.md catalog.

func sdtcliCatalogDataFromRegistry() sdtcliCatalogData {
	return sdtcliCatalogData{
		NewTypes:  ctxTypeHelpText(ctxNewTypes()),
		PathTypes: ctxTypeHelpText(ctxPathTypes()),
		ListTypes: ctxListHelpText(),
	}
}

var instrCLITemplate = templates.Must("instructions/cli.md.tmpl", sdtcliCatalogDataFromRegistry(), nil)
