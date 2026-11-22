//go:build !wasm

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/materialpackage"
)

var materialPackageMermaidFenceRE = regexp.MustCompile("(?ms)^```mermaid(?:[ \\t][^\\n]*)?\\r?\\n(.*?)^```[ \\t]*$")

type materialDiagramRequest struct {
	Diagrams []materialDiagramInput `json:"diagrams"`
}

type materialDiagramInput struct {
	Source  string `json:"source"`
	Caption string `json:"-"`
}

type materialDiagramResponse struct {
	Results []materialDiagramResult `json:"results"`
}

type materialDiagramResult struct {
	SVG   string `json:"svg"`
	Error string `json:"error"`
}

type renderedMaterialDiagram struct {
	Caption string
	SVG     string
}

func packageMaterialDiagrams(root, materialID, markdownPath string, markdown []byte, versions map[string]string) ([]byte, []materialpackage.InputArtifact, error) {
	matches := materialPackageMermaidFenceRE.FindAllSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return markdown, nil, nil
	}
	diagrams, err := materialDiagramInputs(markdown, matches)
	if err != nil {
		return nil, nil, err
	}
	svgs, err := renderMaterialDiagramBatch(root, diagrams)
	if err != nil {
		return nil, nil, err
	}
	return replaceMaterialDiagrams(markdown, matches, svgs, materialID, markdownPath, versions)
}

func materialDiagramInputs(markdown []byte, matches [][]int) ([]materialDiagramInput, error) {
	diagrams := make([]materialDiagramInput, 0, len(matches))
	for i, match := range matches {
		caption := materialDiagramCaption(markdown[:match[0]])
		if caption == "" {
			return nil, fmt.Errorf("diagram %d has no accessible caption immediately before its Mermaid block", i+1)
		}
		diagrams = append(diagrams, materialDiagramInput{Source: string(markdown[match[2]:match[3]]), Caption: caption})
	}
	return diagrams, nil
}

func materialDiagramCaption(before []byte) string {
	lines := strings.Split(strings.TrimRight(string(before), "\r\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "<!--") {
			continue
		}
		line = strings.Trim(line, " :：")
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "![") {
			return ""
		}
		return line
	}
	return ""
}

func renderMaterialDiagramBatch(root string, diagrams []materialDiagramInput) ([]renderedMaterialDiagram, error) {
	runtime, err := findMaterialDiagramRuntime()
	if err != nil {
		return nil, err
	}
	webRoot := filepath.Join(root, "web")
	if _, err := os.Stat(filepath.Join(webRoot, "node_modules", "pele", "package.json")); err != nil {
		return nil, fmt.Errorf("diagram rendering skipped: pinned Pele 1.13.0 is unavailable under web/node_modules")
	}
	request, err := json.Marshal(materialDiagramRequest{Diagrams: diagrams})
	if err != nil {
		return nil, fmt.Errorf("encode Mermaid render request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := materialDiagramRuntimeArgs(runtime)
	// #nosec G204 -- runtime is selected by LookPath from fixed bun/node names; arguments are a fixed local script, source arrives only on stdin.
	cmd := exec.CommandContext(ctx, runtime, args...)
	cmd.Dir = webRoot
	cmd.Stdin = bytes.NewReader(request)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("diagram rendering timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("diagram rendering skipped: %s failed: %w", filepath.Base(runtime), err)
	}
	var response materialDiagramResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("diagram renderer returned invalid JSON: %w", err)
	}
	if len(response.Results) != len(diagrams) {
		return nil, fmt.Errorf("diagram renderer returned %d results for %d diagrams", len(response.Results), len(diagrams))
	}
	results := make([]renderedMaterialDiagram, len(response.Results))
	for i, result := range response.Results {
		if result.Error != "" {
			return nil, fmt.Errorf("diagram %d failed to parse/render: %s", i+1, result.Error)
		}
		if !strings.HasPrefix(strings.TrimSpace(result.SVG), "<svg") {
			return nil, fmt.Errorf("diagram %d renderer returned no SVG", i+1)
		}
		results[i] = renderedMaterialDiagram{Caption: diagrams[i].Caption, SVG: result.SVG}
	}
	return results, nil
}

func findMaterialDiagramRuntime() (string, error) {
	for _, name := range []string{"bun", "node"} {
		if runtime, err := exec.LookPath(name); err == nil {
			return runtime, nil
		}
	}
	return "", fmt.Errorf("diagram rendering skipped: neither bun nor node is available")
}

func materialDiagramRuntimeArgs(_ string) []string {
	return []string{"scripts/render-mermaid.mjs"}
}

func replaceMaterialDiagrams(markdown []byte, matches [][]int, diagrams []renderedMaterialDiagram, materialID, markdownPath string, versions map[string]string) ([]byte, []materialpackage.InputArtifact, error) {
	if len(matches) != len(diagrams) {
		return nil, nil, fmt.Errorf("cannot replace %d Mermaid blocks with %d rendered diagrams", len(matches), len(diagrams))
	}
	var output bytes.Buffer
	artifacts := make([]materialpackage.InputArtifact, 0, len(matches))
	last := 0
	for i, match := range matches {
		output.Write(markdown[last:match[0]])
		assetPath := filepath.ToSlash(filepath.Join("assets", "diagrams", materialID, fmt.Sprintf("diagram-%d.svg", i+1)))
		relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(markdownPath)), filepath.FromSlash(assetPath))
		if err != nil {
			return nil, nil, fmt.Errorf("make diagram link relative: %w", err)
		}
		output.WriteString("![")
		output.WriteString(strings.ReplaceAll(strings.ReplaceAll(diagrams[i].Caption, "[", ""), "]", ""))
		output.WriteString("](")
		output.WriteString(filepath.ToSlash(relative))
		output.WriteString(")")
		artifacts = append(artifacts, materialpackage.InputArtifact{
			ID: "diagram:" + materialID + ":" + fmt.Sprint(i+1), Type: "diagram-svg", Path: assetPath,
			Data: []byte(diagrams[i].SVG), SourceVersions: clonePackageVersions(versions),
		})
		last = match[1]
	}
	output.Write(markdown[last:])
	return output.Bytes(), artifacts, nil
}

func clonePackageVersions(versions map[string]string) map[string]string {
	if len(versions) == 0 {
		return nil
	}
	copy := make(map[string]string, len(versions))
	for key, value := range versions {
		copy[key] = value
	}
	return copy
}
