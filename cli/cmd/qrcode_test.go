package cmd

import (
	"os"
	"testing"

	"github.com/sandrolain/sdt/cli/utils"
)

func TestQrCodeGenerate(t *testing.T) {
	data := "Hello World"
	out := execute(t, qrcodeCmd, []byte{}, data)

	str, err := utils.ReadQRCodeImage(out)
	if err != nil {
		t.Fatalf("error reading generate qrcode: %v", err)
	}

	if str != data {
		t.Fatalf("expecting \"%s\", got \"%s\"", data, str)
	}
}

func TestQrCodeRead(t *testing.T) {
	image, err := os.ReadFile("../../test/testqrcode.png")
	if err != nil {
		t.Fatalf("error reading generate qrcode: %v", err)
	}

	out := execute(t, qrcodeReadCmd, image)
	exp := "https://sandrolain.com"
	if exp != string(out) {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}

	image, err = os.ReadFile("../../test/testqrcode.jpg")
	if err != nil {
		t.Fatalf("error reading generate qrcode: %v", err)
	}

	out = execute(t, qrcodeReadCmd, image)
	exp = "https://sandrolain.com"
	if exp != string(out) {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}
