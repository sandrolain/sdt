package cmd

import "github.com/sandrolain/sdt/internal/templates"

// instrCaptureTemplate renders context/instructions/capture.md: the bounded
// self-improvement capture loop with the four-part test and signal routing.
var instrCaptureTemplate = templates.Must("instructions/capture.md.tmpl", nil, nil)