package cmd

import (
	"strings"
	"text/template"
)

// ── role template funcMap ──────────────────────────────────────────────────────
//
// roleFuncMap is the shared func set for the role .tmpl files: register-table
// formatting (shared.md) and source tagging for the project layer (tagged uses
// the (repo)/(assumption) tags, keeping derive-never-invent semantics).

func roleFuncMap() template.FuncMap {
	return template.FuncMap{
		"yesNo": func(b bool) string {
			if b {
				return "yes"
			}
			return "no"
		},
		"ownedList": func(v []string) string {
			if len(v) == 0 {
				return "—"
			}
			return "`" + strings.Join(v, "`, `") + "`"
		},
		"join": func(v []string) string {
			return strings.Join(v, "`, `")
		},
		"tagged": func(fact string) string {
			return tagged(fact, roleTagRepo, roleTagAssumption)
		},
	}
}
