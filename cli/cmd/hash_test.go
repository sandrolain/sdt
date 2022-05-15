package cmd

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestSHA1(t *testing.T) {
	out := execute(t, sha1Cmd, []byte("Hello World!"))
	exp, _ := hex.DecodeString("2ef7bde608ce5404e97d5f042f95f89f1c232871")
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}

func TestSHA256(t *testing.T) {
	out := execute(t, sha256Cmd, []byte("Hello World!"))
	exp, _ := hex.DecodeString("7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069")
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}

func TestSHA384(t *testing.T) {
	out := execute(t, sha384Cmd, []byte("Hello World!"))
	exp, _ := hex.DecodeString("bfd76c0ebbd006fee583410547c1887b0292be76d582d96c242d2a792723e3fd6fd061f9d5cfd13b8f961358e6adba4a")
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}

func TestSHA512(t *testing.T) {
	out := execute(t, sha512Cmd, []byte("Hello World!"))
	exp, _ := hex.DecodeString("861844d6704e8573fec34d967e20bcfef3d424cf48be04e6dc08f2bd58c729743371015ead891cc3cf1c9d34b49264b510751b1ff9e537937bc46b5d6ff4ecc8")
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}

func TestSHAMD5(t *testing.T) {
	out := execute(t, md5Cmd, []byte("Hello World!"))
	exp, _ := hex.DecodeString("ed076287532e86365e841e92bfc50d8c")
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}
