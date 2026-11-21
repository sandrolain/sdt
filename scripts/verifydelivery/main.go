package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type packageJSON struct {
	ImportPath   string
	Dir          string
	Imports      []string
	TestImports  []string
	XTestImports []string
	Incomplete   bool
	Error        *packageErrorJSON
	DepsErrors   []packageErrorJSON
}

type packageErrorJSON struct {
	Err string
}

type gateReport struct {
	Schema  int       `json:"schema"`
	Plan    Selection `json:"selection"`
	Results []Result  `json:"results"`
	Passed  *bool     `json:"passed,omitempty"`
	Outcome string    `json:"outcome"`
}

type Result struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Reason        string   `json:"reason"`
	ElapsedMS     int64    `json:"elapsed_ms"`
	ExitCode      int      `json:"exit_code"`
	Packages      []string `json:"packages,omitempty"`
	Error         string   `json:"error,omitempty"`
	FailureOutput string   `json:"failure_output,omitempty"`
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "verification:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	return runAt(args, os.Stdout, os.Stderr, workingDir)
}

func runAt(args []string, stdout, stderr io.Writer, workingDir string) error {
	flags := flag.NewFlagSet("verify-delivery", flag.ContinueOnError)
	flags.SetOutput(stderr)
	full := flags.Bool("full", false, "run the complete required local check set")
	preview := flags.Bool("preview", false, "show the selected checks without running them")
	baseRef := flags.String("base", "", "comparison ref; defaults to the current branch upstream")
	group := flags.String("group", "", "run one full CI group: test, build, lint, web, vuln, or security")
	format := flags.String("format", "json", "report format: json or text")
	var addedPackages stringList
	flags.Var(&addedPackages, "add-package", "add a product package to local Go checks (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *group != "" && !*full {
		return errors.New("--group requires --full")
	}
	if *format != "json" && *format != "text" {
		return fmt.Errorf("unsupported report format %q", *format)
	}

	root := workingDir
	if actualRoot, rootErr := gitOutput(root, "rev-parse", "--show-toplevel"); rootErr == nil {
		root = strings.TrimSpace(actualRoot)
	}
	if err := os.Chdir(root); err != nil {
		return err
	}

	var packages []Package
	module := ""
	var packageErr error
	selection := Selection{Mode: "full"}
	if *group != "" {
		selection.Mode = "full-group"
		selection.Group = *group
		selection.SelectedChecks = checksForGroup(*group)
		if selection.SelectedChecks == nil {
			return fmt.Errorf("unknown full CI group %q", *group)
		}
	} else if *full {
		selection = selectChecks(root, module, "", nil, packages, nil, true)
	} else {
		packages, module, packageErr = loadPackages(root)
		changes, base, fallback := inventoryChanges(root, *baseRef)
		if packageErr != nil {
			fallback = "Go package inventory unavailable: " + packageErr.Error()
		}
		selection = selectChecks(root, module, base, changes, packages, addedPackages, false)
		if fallback != "" {
			selection.Mode = "full-fallback"
			selection.Fallback = fallback
			selection.SelectedChecks = checksForAll("full fallback: " + fallback)
			selection.DeferredChecks = nil
		}
		if len(addedPackages) > 0 && packageErr != nil {
			return errors.New("cannot validate --add-package because the product package inventory is unavailable")
		}
		if err := validateAddedPackages(addedPackages, packages); err != nil {
			return err
		}
	}
	selection.SelectedChecks = mergeChecks(selection.SelectedChecks)
	if !hasCheck(selection.SelectedChecks, "context-lint") && len(selection.OutOfScopeChecks) == 0 {
		selection.OutOfScopeChecks = []Check{{Name: "context-lint", Status: "out_of_scope", Reason: "no SDT corpus documents changed"}}
	}
	if *group != "" {
		selection.DeferredChecks = deferredGroupChecks(*group)
	}

	report := gateReport{Schema: 1, Plan: selection, Outcome: "preview"}
	if *preview {
		if *format == "json" {
			return writeJSON(stdout, report)
		}
		writePreviewText(stdout, report)
		return nil
	}

	passed := true
	for _, check := range selection.SelectedChecks {
		fmt.Fprintf(stderr, "running: %s — %s\n", check.Name, check.Reason)
		result := executeCheck(root, check, selection.Mode == "full" || selection.Mode == "full-group" || selection.Mode == "full-fallback")
		report.Results = append(report.Results, result)
		fmt.Fprintf(stderr, "%s: %s (%d ms)\n", result.Status, result.Name, result.ElapsedMS)
		if result.Status == "failed" {
			passed = false
		}
	}
	report.Passed = &passed
	report.Outcome = "passed"
	if !passed {
		report.Outcome = "failed"
	}
	if *format == "json" {
		_ = writeJSON(stdout, report)
	} else {
		writeResultText(stdout, report)
	}
	if !passed {
		return errors.New("one or more selected checks failed")
	}
	return nil
}

func inventoryChanges(root, requestedBase string) ([]Change, string, string) {
	baseRef := strings.TrimSpace(requestedBase)
	if baseRef == "" {
		upstream, err := gitOutput(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
		if err != nil {
			return nil, "", "no upstream branch is configured; pass --base or run the full gate"
		}
		baseRef = strings.TrimSpace(upstream)
	}
	base, err := gitOutput(root, "merge-base", baseRef, "HEAD")
	if err != nil {
		return nil, "", "could not find a merge base for " + baseRef
	}
	base = strings.TrimSpace(base)
	diff, err := gitBytes(root, "diff", "--name-status", "-z", "--find-renames", base, "--")
	if err != nil {
		return nil, "", "could not compare the working tree to merge base " + base
	}
	changes, err := parseNameStatus(diff)
	if err != nil {
		return nil, base, "could not parse Git change inventory: " + err.Error()
	}
	untracked, err := gitBytes(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, base, "could not list untracked files"
	}
	for _, path := range bytes.Split(untracked, []byte{0}) {
		if len(path) > 0 {
			changes = append(changes, Change{Status: "?", Path: filepath.ToSlash(string(path))})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Old < changes[j].Old
	})
	return changes, base, ""
}

func parseNameStatus(data []byte) ([]Change, error) {
	fields := bytes.Split(data, []byte{0})
	changes := make([]Change, 0, len(fields)/2)
	for i := 0; i < len(fields); {
		if len(fields[i]) == 0 {
			i++
			continue
		}
		status := string(fields[i])
		i++
		if i >= len(fields) || len(fields[i]) == 0 {
			return nil, errors.New("status entry has no path")
		}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			old := filepath.ToSlash(string(fields[i]))
			i++
			if i >= len(fields) || len(fields[i]) == 0 {
				return nil, errors.New("rename/copy entry has no destination path")
			}
			changes = append(changes, Change{Status: status, Old: old, Path: filepath.ToSlash(string(fields[i]))})
			i++
			continue
		}
		changes = append(changes, Change{Status: status, Path: filepath.ToSlash(string(fields[i]))})
		i++
	}
	return changes, nil
}

func loadPackages(root string) ([]Package, string, error) {
	module, err := gitOrGoOutput(root, "go", "list", "-m")
	if err != nil {
		return nil, "", err
	}
	output, err := goBytes(root, "list", "-e", "-json", "./...")
	if err != nil {
		return nil, module, err
	}
	packages, err := parsePackageInventory(output, module)
	return packages, module, err
}

func parsePackageInventory(output []byte, module string) ([]Package, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	packages := []Package{}
	for {
		var item packageJSON
		if err := decoder.Decode(&item); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if item.ImportPath == module+"/context" || strings.HasPrefix(item.ImportPath, module+"/context/") {
			continue
		}
		if item.ImportPath == "" || item.Dir == "" {
			return nil, errors.New("Go package inventory contains a package without an import path or directory")
		}
		if item.Incomplete || item.Error != nil || len(item.DepsErrors) > 0 {
			return nil, fmt.Errorf("Go package inventory is incomplete for %s", item.ImportPath)
		}
		imports := append(item.Imports, item.TestImports...)
		imports = append(imports, item.XTestImports...)
		packages = append(packages, Package{ImportPath: item.ImportPath, Dir: item.Dir, Imports: uniqueSorted(imports)})
	}
	if len(packages) == 0 {
		return nil, errors.New("Go package inventory is empty")
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].ImportPath < packages[j].ImportPath })
	return packages, nil
}

func validateAddedPackages(additions []string, packages []Package) error {
	for _, input := range additions {
		if resolvePackageName(input, modulePath(packages), packages) == "" {
			return fmt.Errorf("--add-package %q is not a product package", input)
		}
	}
	return nil
}

func checksForGroup(group string) []Check {
	valid := false
	checks := []Check{}
	for _, spec := range fullChecks {
		if spec.group == group || (group == "vuln" && spec.name == "govulncheck") {
			valid = true
			checks = append(checks, Check{Name: spec.name, Status: "selected", Reason: "required by explicit full CI group " + group})
		}
	}
	if !valid {
		return nil
	}
	return checks
}

func deferredGroupChecks(group string) []Check {
	deferred := []Check{}
	for _, spec := range fullChecks {
		if spec.group == group || (group == "vuln" && spec.name == "govulncheck") {
			continue
		}
		deferred = append(deferred, Check{Name: spec.name, Status: "deferred_to_ci_group", Reason: "covered by another full CI group"})
	}
	return deferred
}

func mergeChecks(checks []Check) []Check {
	byName := map[string]Check{}
	for _, check := range checks {
		previous, exists := byName[check.Name]
		if !exists {
			byName[check.Name] = check
			continue
		}
		previous.Packages = append(previous.Packages, check.Packages...)
		previous.Reason = combineReason(previous.Reason, check.Reason)
		byName[check.Name] = previous
	}
	out := make([]Check, 0, len(byName))
	for _, spec := range fullChecks {
		if check, ok := byName[spec.name]; ok {
			check.Packages = uniqueSorted(check.Packages)
			out = append(out, check)
			delete(byName, spec.name)
		}
	}
	if check, ok := byName["context-lint"]; ok {
		out = append(out, check)
	}
	return out
}

func combineReason(first, second string) string {
	if first == second || second == "" {
		return first
	}
	if first == "" {
		return second
	}
	return first + "; " + second
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func executeCheck(root string, check Check, full bool) Result {
	result := Result{Name: check.Name, Status: "passed", Reason: check.Reason, Packages: check.Packages}
	started := time.Now()
	if check.Name == "format" && !full {
		files, err := goFilesForPackages(check.Packages)
		if err == nil {
			cmd := exec.Command("gofmt", append([]string{"-l"}, files...)...) // #nosec G204 -- executable is fixed; file paths come from go list.
			cmd.Dir = root
			var output []byte
			output, err = cmd.Output()
			if len(output) > 0 {
				err = fmt.Errorf("unformatted Go files:\n%s", strings.TrimSpace(string(output)))
				result.FailureOutput = persistFailureOutput(root, output)
			}
		}
		finishResult(&result, started, err)
		return result
	}
	args, err := commandFor(check, full)
	if err == nil {
		cmd := exec.CommandContext(context.Background(), args[0], args[1:]...) // #nosec G204 -- commandFor returns a fixed allowlist; no shell parses its arguments.
		cmd.Dir = root
		capture := &tailBuffer{limit: 64 * 1024}
		tee := io.MultiWriter(os.Stderr, capture)
		cmd.Stdout = tee
		cmd.Stderr = tee
		err = cmd.Run()
		if err != nil && capture.Len() > 0 {
			result.FailureOutput = persistFailureOutput(root, capture.Bytes())
		}
	}
	finishResult(&result, started, err)
	return result
}

func finishResult(result *Result, started time.Time, err error) {
	result.ElapsedMS = time.Since(started).Milliseconds()
	if err == nil {
		return
	}
	result.Status = "failed"
	result.Error = err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = 1
	}
}

type tailBuffer struct {
	data  []byte
	limit int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(p), nil
}
func (b *tailBuffer) Bytes() []byte { return b.data }
func (b *tailBuffer) Len() int      { return len(b.data) }

func persistFailureOutput(root string, output []byte) string {
	if len(output) == 0 {
		return ""
	}
	dir := filepath.Join(root, "context", "tmp")
	if os.MkdirAll(dir, 0o700) != nil {
		return ""
	}
	file, err := os.CreateTemp(dir, "verify-delivery-failure-*.log")
	if err != nil {
		return ""
	}
	defer file.Close()
	if file.Chmod(0o600) != nil {
		_ = os.Remove(file.Name())
		return ""
	}
	if _, err := file.Write(output); err != nil {
		_ = os.Remove(file.Name())
		return ""
	}
	relative, err := filepath.Rel(root, file.Name())
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative)
}

func commandFor(check Check, full bool) ([]string, error) {
	if full {
		switch check.Name {
		case "format":
			return []string{"sh", "scripts/verify.sh", "fmt"}, nil
		case "modules":
			return []string{"sh", "scripts/verify.sh", "modules"}, nil
		case "entry-metadata":
			return []string{"sh", "scripts/verify.sh", "entry-metadata"}, nil
		case "delivery-tool-test":
			return []string{"sh", "scripts/verify.sh", "delivery-tool-test"}, nil
		case "build":
			return []string{"sh", "scripts/verify.sh", "build"}, nil
		case "vet":
			return []string{"sh", "scripts/verify.sh", "vet"}, nil
		case "lint":
			return []string{"sh", "scripts/verify.sh", "lint"}, nil
		case "test":
			return []string{"sh", "scripts/verify.sh", "test"}, nil
		case "coverage":
			return []string{"sh", "scripts/verify.sh", "coverage"}, nil
		case "web":
			return []string{"sh", "-c", "cd web && bun run lint && bun run fmt:check && bun run test && bun run build"}, nil
		case "govulncheck":
			return []string{"sh", "scripts/verify.sh", "vuln"}, nil
		case "gosec":
			return []string{"sh", "-c", "gosec -quiet -severity medium -exclude-dir=context ./cli/... ./internal/... ./viewer/... && (cd scripts/verifydelivery && gosec -quiet -severity medium .)"}, nil
		case "trivy":
			return []string{"trivy", "fs", "--scanners", "vuln,secret,misconfig", "--severity", "CRITICAL,HIGH", "--exit-code", "1", "--skip-version-check", "--no-progress", "--skip-dirs", "node_modules", "--skip-dirs", "context", "--skip-dirs", "bin", "."}, nil
		case "context-lint":
			return []string{"sdt", "context", "lint"}, nil
		default:
			return nil, fmt.Errorf("unknown full check %q", check.Name)
		}
	}
	if check.Name == "context-lint" {
		return []string{"sdt", "context", "lint"}, nil
	}
	if check.Name == "web" {
		return []string{"sh", "-c", "cd web && bun run lint && bun run fmt:check && bun run test && bun run build"}, nil
	}
	if check.Name == "entry-metadata" {
		return []string{"sh", "scripts/verify.sh", "entry-metadata"}, nil
	}
	if len(check.Packages) == 0 {
		return nil, fmt.Errorf("selected check %q has no package scope", check.Name)
	}
	args := append([]string{}, check.Packages...)
	switch check.Name {
	case "format":
		files, err := goFilesForPackages(check.Packages)
		if err != nil {
			return nil, err
		}
		return append([]string{"gofmt", "-l"}, files...), nil
	case "build":
		return append([]string{"go", "build"}, args...), nil
	case "vet":
		return append([]string{"go", "vet"}, args...), nil
	case "lint":
		return append([]string{"golangci-lint", "run"}, args...), nil
	case "test":
		return append([]string{"go", "test"}, args...), nil
	case "gosec":
		return append([]string{"gosec", "-quiet", "-severity", "medium", "-exclude-dir=context"}, args...), nil
	default:
		return nil, fmt.Errorf("check %q cannot run with a targeted package scope", check.Name)
	}
}

func goFilesForPackages(packages []string) ([]string, error) {
	args := append([]string{"list", "-f", "{{.Dir}}"}, packages...)
	output, err := goBytes(".", args...)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, dir := range strings.Fields(string(output)) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		return nil, errors.New("selected Go packages have no source files")
	}
	sort.Strings(files)
	return files, nil
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writePreviewText(out io.Writer, report gateReport) {
	fmt.Fprintf(out, "mode: %s\nbase: %s\n", report.Plan.Mode, valueOr(report.Plan.Base, "unavailable"))
	if report.Plan.Group != "" {
		fmt.Fprintf(out, "group: %s (complete group; other checks are listed below)\n", report.Plan.Group)
	}
	if report.Plan.Fallback != "" {
		fmt.Fprintf(out, "fallback: %s\n", report.Plan.Fallback)
	}
	for _, change := range report.Plan.ChangedFiles {
		if change.Old != "" {
			fmt.Fprintf(out, "changed: %s %s -> %s\n", change.Status, change.Old, change.Path)
		} else {
			fmt.Fprintf(out, "changed: %s %s\n", change.Status, change.Path)
		}
	}
	for _, check := range report.Plan.SelectedChecks {
		fmt.Fprintf(out, "selected: %s — %s", check.Name, check.Reason)
		if len(check.Packages) > 0 {
			fmt.Fprintf(out, " (%s)", strings.Join(check.Packages, ", "))
		}
		fmt.Fprintln(out)
	}
	for _, check := range report.Plan.DeferredChecks {
		fmt.Fprintf(out, "deferred: %s — %s\n", check.Name, check.Reason)
	}
	for _, check := range report.Plan.OutOfScopeChecks {
		fmt.Fprintf(out, "out of scope: %s — %s\n", check.Name, check.Reason)
	}
	if len(report.Plan.AgentPackages) > 0 {
		fmt.Fprintf(out, "agent-added packages: %s\n", strings.Join(report.Plan.AgentPackages, ", "))
	}
	fmt.Fprintln(out, "selection preview only; no checks run")
}

func writeResultText(out io.Writer, report gateReport) {
	fmt.Fprintf(out, "mode: %s\nbase: %s\n", report.Plan.Mode, valueOr(report.Plan.Base, "unavailable"))
	if report.Plan.Fallback != "" {
		fmt.Fprintf(out, "fallback: %s\n", report.Plan.Fallback)
	}
	for _, check := range report.Plan.SelectedChecks {
		fmt.Fprintf(out, "selected: %s — %s", check.Name, check.Reason)
		if len(check.Packages) > 0 {
			fmt.Fprintf(out, " (%s)", strings.Join(check.Packages, ", "))
		}
		fmt.Fprintln(out)
	}
	for _, result := range report.Results {
		fmt.Fprintf(out, "%s: %s (%d ms)", result.Name, result.Status, result.ElapsedMS)
		if result.Error != "" {
			fmt.Fprintf(out, " — %s", result.Error)
		}
		if result.FailureOutput != "" {
			fmt.Fprintf(out, " (output: %s)", result.FailureOutput)
		}
		fmt.Fprintln(out)
	}
	for _, check := range report.Plan.DeferredChecks {
		fmt.Fprintf(out, "deferred: %s — %s\n", check.Name, check.Reason)
	}
	for _, check := range report.Plan.OutOfScopeChecks {
		fmt.Fprintf(out, "out of scope: %s — %s\n", check.Name, check.Reason)
	}
	if len(report.Plan.AgentPackages) > 0 {
		fmt.Fprintf(out, "agent-added packages: %s\n", strings.Join(report.Plan.AgentPackages, ", "))
	}
	if report.Outcome == "failed" {
		fmt.Fprintln(out, "delivery verification: FAILED")
	} else if report.Outcome == "passed" {
		fmt.Fprintln(out, "delivery verification: PASSED")
	} else {
		fmt.Fprintln(out, "delivery verification: PREVIEW (no checks run)")
	}
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func gitOutput(root string, args ...string) (string, error) {
	output, err := gitBytes(root, args...)
	return string(output), err
}

func gitBytes(root string, args ...string) ([]byte, error) {
	return commandOutput(root, "git", args...)
}

func gitOrGoOutput(root, command string, args ...string) (string, error) {
	output, err := commandOutput(root, command, args...)
	return strings.TrimSpace(string(output)), err
}

func goBytes(root string, args ...string) ([]byte, error) {
	return commandOutput(root, "go", args...)
}

func commandOutput(root, command string, args ...string) ([]byte, error) {
	cmd := exec.Command(command, args...) // #nosec G204 -- callers pass only the fixed git/go executables.
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return output, nil
}
