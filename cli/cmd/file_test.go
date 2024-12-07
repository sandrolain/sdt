package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFileExists(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "file_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
		wantErr  bool
	}{
		{
			name:     "Existing file",
			path:     tmpFile,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "Non-existent file",
			path:     filepath.Join(tmpDir, "nonexistent.txt"),
			expected: false,
			wantErr:  false,
		},
		{
			name:     "Empty path",
			path:     "",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "Existing directory",
			path:     tmpDir,
			expected: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := fileExists(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("fileExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if exists != tt.expected {
				t.Errorf("fileExists() = %v, expected %v", exists, tt.expected)
			}
		})
	}
}

func TestFileReadCmd(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "file_read_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file with content
	testContent := "test file content"
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tests := []struct {
		name      string
		filePath  string
		wantError bool
	}{
		{
			name:      "Existing file",
			filePath:  tmpFile,
			wantError: false,
		},
		{
			name:      "Non-existent file",
			filePath:  filepath.Join(tmpDir, "nonexistent.txt"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			_ = cmd.Flags().String("file", "", "File path")
			_ = cmd.Flags().String("output", "raw", "Output format")
			_ = cmd.Flags().Set("file", tt.filePath)

			// Capture output and error
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			// Redirect os.Exit calls to panic for testing
			oldOsExit := exit
			defer func() { exit = oldOsExit }()

			var exitCode int
			exit = func(code int) {
				exitCode = code
				panic("os.Exit called")
			}

			if tt.wantError {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected the command to exit with error")
					}
					if exitCode != 1 {
						t.Errorf("Expected exit code 1, got %d", exitCode)
					}
					expectedError := fmt.Sprintf("file %q not exist", tt.filePath)
					if !strings.Contains(stderr.String(), expectedError) {
						t.Errorf("Expected error message %q, got %q", expectedError, stderr.String())
					}
				}()
			}

			fileReadCmd.Run(cmd, nil)

			if !tt.wantError && stderr.Len() > 0 {
				t.Errorf("Unexpected error output: %s", stderr.String())
			}
		})
	}
}
