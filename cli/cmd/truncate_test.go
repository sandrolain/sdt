package cmd

import (
	"strings"
	"testing"
)

func TestTruncateToTokens_noop(t *testing.T) {
	text := "Hello"
	result := truncateToTokens(text, "cl100k", 100)
	if result != text {
		t.Errorf("expected unchanged text, got %q", result)
	}
}

func TestTruncateToTokens_truncates(t *testing.T) {
	// Build a long text with a predictable token count.
	words := strings.Repeat("word ", 200)
	result := truncateToTokens(words, "cl100k", 10)
	count := CountTokens(result, "cl100k")
	if count > 10 {
		t.Errorf("expected <= 10 tokens after truncation, got %d", count)
	}
	if result == "" {
		t.Error("expected non-empty truncation result")
	}
}

func TestTruncateSentences(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence which is very very long and makes it exceed."
	result := truncateSentences(text, "cl100k", 5)
	count := CountTokens(result, "cl100k")
	if count > 5 {
		t.Errorf("expected <= 5 tokens, got %d", count)
	}
}

func TestTruncateSections(t *testing.T) {
	text := "# Section 1\n\nContent of section one.\n\n# Section 2\n\nContent of section two.\n\n# Section 3\n\nContent of section three."
	result := truncateSections(text, "cl100k", 10)
	count := CountTokens(result, "cl100k")
	if count > 10 {
		t.Errorf("expected <= 10 tokens, got %d", count)
	}
}

func TestTruncateCmd_default(t *testing.T) {
	long := strings.Repeat("word ", 500)
	out := execute(t, truncateCmd, []byte(long), "--max-tokens", "20")
	count := CountTokens(string(out), "cl100k")
	if count > 22 { // allow small trailing newline
		t.Errorf("expected <= 20 tokens in output, got %d", count)
	}
}

func TestTruncateCmd_strategy_sentences(t *testing.T) {
	text := "Hello world. This is a test sentence. Another sentence follows here."
	out := execute(t, truncateCmd, []byte(text), "--max-tokens", "5", "--strategy", "sentences")
	count := CountTokens(string(out), "cl100k")
	if count > 5 {
		t.Errorf("expected <= 5 tokens, got %d", count)
	}
}

func TestTruncateCmd_invalidMaxTokens(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, truncateCmd, []byte("hello"), "--max-tokens", "0")
		return ""
	})
}
