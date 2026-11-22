package cmd

import (
	"fmt"
	"strings"
)

const privacyNone = "none"

var privacyValues = []string{privacyNone, "personal", "real"}

func lintPrivacyDeclaration(path, content string) []ctxLintIssue {
	fields := learningMetadataFields(content)
	value, declared := fields[ctxFrontmatterPrivacy]
	clearance, hasClearance := fields["privacy_clearance"]
	if !declared && !hasClearance {
		return nil
	}
	privacy, ok := value.(string)
	if !ok || strings.TrimSpace(privacy) == "" {
		return privacyFinding(path, "`privacy` must declare none, personal or real")
	}
	if !containsItem(privacyValues, privacy) {
		return privacyFinding(path, fmt.Sprintf("`privacy` %q is outside the vocabulary (%s)", privacy, strings.Join(privacyValues, " | ")))
	}
	if privacy == privacyNone {
		if hasClearance {
			return privacyFinding(path, "`privacy_clearance` is not valid when `privacy` is none")
		}
		return nil
	}
	clearanceFields, ok := clearance.(map[string]any)
	if !ok || clearanceFields["status"] != "cleared" || strings.TrimSpace(fmt.Sprint(clearanceFields["basis"])) == "" || clearanceFields["basis"] == "<nil>" {
		return privacyFinding(path, "`privacy` personal or real requires `privacy_clearance.status: cleared` and a short `basis`")
	}
	return nil
}

func privacyFinding(path, message string) []ctxLintIssue {
	return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: message}}
}
