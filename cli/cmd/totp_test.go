package cmd

import (
	"encoding/base32"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/sandrolain/sdt/cli/utils"
)

func TestTotpUri(t *testing.T) {
	secret := base32.StdEncoding.EncodeToString([]byte("hello/world"))
	issuer := "sdt.test"
	account := "user@sdt"
	out := execute(t, totpUriCmd, []byte{}, "--secret", secret, "--issuer", issuer, "--account", account)
	exp := "otpauth://totp/sdt.test:user@sdt?algorithm=SHA1&digits=6&issuer=sdt.test&period=30&secret=NBSWY3DPF53W64TMMQ"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestTotpImage(t *testing.T) {
	secret := base32.StdEncoding.EncodeToString([]byte("hello/world"))
	issuer := "sdt.test"
	account := "user@sdt"
	outBytes := execute(t, totpImageCmd, []byte{}, "--secret", secret, "--issuer", issuer, "--account", account)
	out, err := utils.ReadQRCodeImage(outBytes)
	if err != nil {
		t.Fatal(err)
	}
	exp := "otpauth://totp/sdt.test:user@sdt?algorithm=SHA1&digits=6&issuer=sdt.test&period=30&secret=NBSWY3DPF53W64TMMQ"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestTotpCode(t *testing.T) {
	secret := base32.StdEncoding.EncodeToString([]byte("hello/world"))
	out := execute(t, totpCodeCmd, []byte{}, "--secret", secret)
	exp, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestTotpVerify(t *testing.T) {
	secret := base32.StdEncoding.EncodeToString([]byte("hello/world"))
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	out := execute(t, totpVerifyCmd, []byte{}, "--secret", secret, "--code", code)
	if string(out) != code {
		t.Fatalf("expecting \"%s\", got \"%s\"", code, string(out))
	}

	code = "000000"
	shouldExitWithCode(t, 1, func() string {
		out := execute(t, totpVerifyCmd, []byte{}, "--secret", secret, "--code", code)
		return string(out)
	})
}
