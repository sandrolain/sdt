package cmd

import "github.com/sandrolain/sdt/internal/templates"

// categoriesRegisterTemplate seeds context/categories.yaml: the controlled
// vocabulary for the `categories` frontmatter field (the kind of work an
// analysis is). Canonical categories are kebab-case slugs; aliases are accepted
// by lint and canonicalized. Generated at init, user-editable thereafter.

var categoriesRegisterTemplate = templates.Must("workspace/categories.yaml.tmpl", nil, nil)
