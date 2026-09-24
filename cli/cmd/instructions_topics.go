package cmd

import "github.com/sandrolain/sdt/internal/templates"

// topicsRegisterTemplate seeds context/topics.yaml: the controlled vocabulary
// for the `topics` frontmatter field. Canonical topics are kebab-case slugs;
// aliases are accepted by lint and canonicalized in hints. It is generated at
// init and, like the rest of the workspace, user-editable thereafter.

var topicsRegisterTemplate = templates.Must("workspace/topics.yaml.tmpl", nil, nil)
