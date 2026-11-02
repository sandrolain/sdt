package cmd

import (
	"github.com/sandrolain/sdt/internal/feedback"
)

// validateFeedbackRegister checks the portal-feedback register: the closed
// vocabularies, the sample floor and a reason on every closed entry. The register
// is the record — no command writes it and no projection is generated today,
// because the CLI declares no feedback verb.
func validateFeedbackRegister() error {
	_, err := feedback.Load(".")
	return err
}
