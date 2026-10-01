// Package doc2md converts a local document to Markdown through the first
// runnable external converter in a fixed order (anydoc, docling, markitdown).
//
// Availability means "launches", not "is on PATH": a converter is probed with
// an exec run, never with a PATH lookup, so an installed-but-broken tool is
// reported instead of silently selected. A converter that fails to launch,
// exits non-zero, times out or produces no markdown advances the chain with a
// reason; the first success wins.
//
// The package touches the OS only through the injectable execCommand seam, so
// tests never need anydoc, docling or markitdown to be installed.
package doc2md

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTimeout bounds one probe or conversion process.
const DefaultTimeout = 120 * time.Second

// Status is the normalized outcome of a conversion.
type Status string

const (
	StatusSuccess     Status = "success"
	StatusNeedsOCR    Status = "needs_ocr"
	StatusUnavailable Status = "unavailable"
	StatusFailed      Status = "failed"
)

// ExitCode maps a status to the command exit code: 0 success, 3 needs_ocr
// (mirroring anydoc's own exit code), 2 unavailable, 1 failed.
func (s Status) ExitCode() int {
	switch s {
	case StatusSuccess:
		return 0
	case StatusNeedsOCR:
		return 3
	case StatusUnavailable:
		return 2
	default:
		return 1
	}
}

// Stage identifies where a converter attempt stopped.
type Stage string

const (
	StageProbe   Stage = "probe"
	StageConvert Stage = "convert"
)

// Attempt records one converter trial in the chain.
type Attempt struct {
	Converter string `json:"converter" yaml:"converter"`
	Stage     Stage  `json:"stage" yaml:"stage"`
	Reason    string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// Result is the outcome of Convert.
type Result struct {
	Status    Status    `json:"status" yaml:"status"`
	Converter string    `json:"converter,omitempty" yaml:"converter,omitempty"`
	Markdown  string    `json:"markdown,omitempty" yaml:"markdown,omitempty"`
	Reason    string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	KeptFile  string    `json:"kept_file,omitempty" yaml:"kept_file,omitempty"`
	Attempts  []Attempt `json:"attempts" yaml:"attempts"`
}

// Options configures Convert.
type Options struct {
	// Input is the local path of the document to convert.
	Input string
	// Timeout bounds one probe or conversion; DefaultTimeout when zero.
	Timeout time.Duration
	// Tool pins the chain to one converter by name.
	Tool string
}

// ToolStatus is one row of the tools probe table.
type ToolStatus struct {
	Name      string `json:"name" yaml:"name"`
	Available bool   `json:"available" yaml:"available"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
	Reason    string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// ExecResult captures one finished process. Err carries launch, timeout or
// cancellation failures; a non-zero exit is reported through ExitCode.
type ExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

// execCommand is the exec seam; tests replace it with a fake.
var execCommand = runCommand

func runCommand(ctx context.Context, name string, args ...string) ExecResult {
	//#nosec G204 -- argv comes from the closed converter table below, never a
	// user-supplied command string and never a shell.
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := ExecResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err == nil {
		return res
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		res.ExitCode = -1
		res.Err = ctxErr
		return res
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res
	}
	res.ExitCode = -1
	res.Err = err
	return res
}

type converter struct {
	name    string
	convert func(ctx context.Context, input string) (markdown []byte, needsOCR bool, err error)
}

// converters is the fixed preference order. The names are the only source of
// tool names in the package; nothing is ever resolved from user input beyond
// --tool matching one of these.
var converters = []converter{
	{name: "anydoc", convert: convertAnydoc},
	{name: "docling", convert: convertDocling},
	{name: "markitdown", convert: convertMarkitdown},
}

// Names returns the converter names in chain order.
func Names() []string {
	names := make([]string, 0, len(converters))
	for _, c := range converters {
		names = append(names, c.name)
	}
	return names
}

// Tools probes every converter and reports availability. A tool counts as
// available only when it launches and answers a version or help probe.
func Tools(ctx context.Context, timeout time.Duration) []ToolStatus {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	out := make([]ToolStatus, 0, len(converters))
	for _, c := range converters {
		status := ToolStatus{Name: c.name}
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		version, err := probe(probeCtx, c.name)
		cancel()
		if err != nil {
			status.Reason = err.Error()
		} else {
			status.Available = true
			status.Version = version
		}
		out = append(out, status)
	}
	return out
}

// Convert tries the chain in order and returns the first success. A probe
// failure or an empty/failed conversion advances the chain; an OCR-needed PDF
// stops the chain and is reported as needs_ocr rather than flattened into a
// generic failure.
func Convert(ctx context.Context, opts Options) Result {
	result := Result{Attempts: []Attempt{}}
	if strings.TrimSpace(opts.Input) == "" {
		result.Status = StatusFailed
		result.Reason = "no input document"
		return result
	}

	chain := converters
	if opts.Tool != "" {
		chain = nil
		for _, c := range converters {
			if c.name == opts.Tool {
				chain = []converter{c}
				break
			}
		}
		if chain == nil {
			result.Status = StatusFailed
			result.Reason = fmt.Sprintf("unknown tool %q (available: %s)", opts.Tool, strings.Join(Names(), ", "))
			return result
		}
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	probed := false
	for _, c := range chain {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		_, probeErr := probe(runCtx, c.name)
		if probeErr != nil {
			result.Attempts = append(result.Attempts, Attempt{Converter: c.name, Stage: StageProbe, Reason: probeErr.Error()})
			cancel()
			continue
		}
		probed = true

		markdown, needsOCR, err := c.convert(runCtx, opts.Input)
		cancel()
		switch {
		case needsOCR:
			reason := err.Error()
			result.Status = StatusNeedsOCR
			result.Converter = c.name
			result.Reason = reason
			result.Attempts = append(result.Attempts, Attempt{Converter: c.name, Stage: StageConvert, Reason: reason})
			return result
		case err != nil:
			result.Attempts = append(result.Attempts, Attempt{Converter: c.name, Stage: StageConvert, Reason: err.Error()})
		default:
			result.Status = StatusSuccess
			result.Converter = c.name
			result.Markdown = string(markdown)
			result.Attempts = append(result.Attempts, Attempt{Converter: c.name, Stage: StageConvert})
			return result
		}
	}

	if !probed {
		result.Status = StatusUnavailable
		result.Reason = fmt.Sprintf("no runnable converter (%s); install one, or run it explicitly with uvx (e.g. \"uvx --from markitdown markitdown\")",
			strings.Join(chainNames(chain), ", "))
		return result
	}
	result.Status = StatusFailed
	result.Reason = lastFailureReason(result.Attempts)
	return result
}

// lastFailureReason prefers the last conversion failure over a probe failure:
// once a converter ran, that is the more useful reason to report.
func lastFailureReason(attempts []Attempt) string {
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Stage == StageConvert {
			return attempts[i].Reason
		}
	}
	if n := len(attempts); n > 0 {
		return attempts[n-1].Reason
	}
	return ""
}

func chainNames(chain []converter) []string {
	names := make([]string, 0, len(chain))
	for _, c := range chain {
		names = append(names, c.name)
	}
	return names
}

// probe runs a launch test: --version first, then --help for CLIs that do not
// implement --version. A zero exit on either proves the binary launches.
func probe(ctx context.Context, name string) (string, error) {
	version := execCommand(ctx, name, "--version")
	if version.Err == nil && version.ExitCode == 0 {
		return firstLine(version.Stdout), nil
	}
	help := execCommand(ctx, name, "--help")
	if help.Err == nil && help.ExitCode == 0 {
		return "", nil
	}
	if version.Err != nil {
		return "", version.Err
	}
	if help.Err != nil {
		return "", help.Err
	}
	reason := reasonFromOutput(version.Stderr)
	if reason == "" {
		reason = reasonFromOutput(help.Stderr)
	}
	if reason == "" {
		reason = fmt.Sprintf("probe exited %d", version.ExitCode)
	}
	return "", errors.New(reason)
}

func convertAnydoc(ctx context.Context, input string) ([]byte, bool, error) {
	res := execCommand(ctx, "anydoc", input)
	if res.Err != nil {
		return nil, false, res.Err
	}
	if res.ExitCode == 3 {
		return nil, true, errors.New("PDF needs OCR (pass --ocr hosted on anydoc, or install an OCR-capable converter)")
	}
	if res.ExitCode != 0 {
		return nil, false, fmt.Errorf("anydoc exited %d: %s", res.ExitCode, reasonFromOutput(res.Stderr))
	}
	return nonEmptyMarkdown(res.Stdout)
}

func convertMarkitdown(ctx context.Context, input string) ([]byte, bool, error) {
	res := execCommand(ctx, "markitdown", input)
	if res.Err != nil {
		return nil, false, res.Err
	}
	if res.ExitCode != 0 {
		return nil, false, fmt.Errorf("markitdown exited %d: %s", res.ExitCode, reasonFromOutput(res.Stderr))
	}
	return nonEmptyMarkdown(res.Stdout)
}

func convertDocling(ctx context.Context, input string) ([]byte, bool, error) {
	dir, err := os.MkdirTemp("", "sdt-doc2md-*")
	if err != nil {
		return nil, false, fmt.Errorf("create docling output dir: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of the adapter's temp dir

	res := execCommand(ctx, "docling", "convert", input, "--to", "md", "--output", dir)
	if res.Err != nil {
		return nil, false, res.Err
	}
	if res.ExitCode != 0 {
		return nil, false, fmt.Errorf("docling exited %d: %s", res.ExitCode, reasonFromOutput(res.Stderr))
	}
	out := filepath.Join(dir, sourceStem(input)+".md")
	//#nosec G304 -- the path is the adapter's own temp dir plus the input stem
	data, err := os.ReadFile(out)
	if err != nil {
		return nil, false, fmt.Errorf("docling produced no markdown at %s", out)
	}
	return nonEmptyMarkdown(data)
}

func nonEmptyMarkdown(data []byte) ([]byte, bool, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, false, errors.New("converter produced no markdown")
	}
	return data, false, nil
}

func sourceStem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func firstLine(data []byte) string {
	s := strings.TrimSpace(string(data))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// reasonFromOutput picks the most useful line of a tool's stderr: the first
// line that names an error, otherwise the first non-empty line (a Python
// traceback's useful line is its last, an asdf shim's is its first).
func reasonFromOutput(data []byte) string {
	first, best := "", ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if first == "" {
			first = line
		}
		if best == "" && namesError(line) {
			best = line
		}
	}
	if best != "" {
		return clip(best)
	}
	return clip(first)
}

func namesError(line string) bool {
	lower := strings.ToLower(line)
	for _, kw := range []string{"error", "not found", "no version", "unavailable", "cannot", "failed"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
