package cmd

import (
	"strings"
	"testing"
)

func TestVManMajor(t *testing.T) {
	out := execute(t, vmajCmd, []byte("1.2.3"))
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected 1, got %s", out)
	}
}

func TestVManMinor(t *testing.T) {
	out := execute(t, vminCmd, []byte("1.2.3"))
	if strings.TrimSpace(string(out)) != "2" {
		t.Errorf("expected 2, got %s", out)
	}
}

func TestVManPatch(t *testing.T) {
	out := execute(t, vpatCmd, []byte("1.2.3"))
	if strings.TrimSpace(string(out)) != "3" {
		t.Errorf("expected 3, got %s", out)
	}
}

func TestVManMajor_increment(t *testing.T) {
	out := execute(t, vmajCmd, []byte("1.2.3"), "--action", "++")
	if strings.TrimSpace(string(out)) != "2.2.3" {
		t.Errorf("expected 2.2.3, got %s", out)
	}
}

func TestVManMinor_decrement(t *testing.T) {
	out := execute(t, vminCmd, []byte("1.2.3"), "--action", "--")
	if strings.TrimSpace(string(out)) != "1.1.3" {
		t.Errorf("expected 1.1.3, got %s", out)
	}
}

func TestVManPrerelease(t *testing.T) {
	out := execute(t, vpreCmd, []byte("1.2.3-alpha"))
	// existing code prints prerelease value then full version (no early return)
	if !strings.Contains(string(out), "alpha") {
		t.Errorf("expected alpha in output, got %s", out)
	}
}

func TestVManPrerelease_set(t *testing.T) {
	out := execute(t, vpreCmd, []byte("1.2.3"), "--action", "beta")
	if !strings.Contains(string(out), "beta") {
		t.Errorf("expected beta in output, got %s", out)
	}
}

func TestVManPrerelease_remove(t *testing.T) {
	out := execute(t, vpreCmd, []byte("1.2.3-alpha"), "--action", "--")
	if strings.Contains(string(out), "alpha") {
		t.Errorf("expected alpha removed, got %s", out)
	}
}

func TestVManMetadata(t *testing.T) {
	out := execute(t, vmetCmd, []byte("1.2.3+build42"))
	// existing code prints metadata value then full version (no early return)
	if !strings.Contains(string(out), "build42") {
		t.Errorf("expected build42 in output, got %s", out)
	}
}

func TestVManMetadata_set(t *testing.T) {
	out := execute(t, vmetCmd, []byte("1.2.3"), "--action", "newmeta")
	if !strings.Contains(string(out), "newmeta") {
		t.Errorf("expected newmeta in output, got %s", out)
	}
}

func TestVManMetadata_remove(t *testing.T) {
	out := execute(t, vmetCmd, []byte("1.2.3+meta"), "--action", "--")
	if strings.Contains(string(out), "meta") {
		t.Errorf("expected meta removed, got %s", out)
	}
}
