package cmd

import "github.com/sandrolain/sdt/internal/templates"

type projectTemplateData struct {
	Project string
	Group   string
}

func instrProjectTemplate(project, group string) string {
	return templates.Must("instructions/project.md.tmpl", projectTemplateData{Project: project, Group: group}, nil)
}
