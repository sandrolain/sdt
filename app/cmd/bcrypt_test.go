package cmd

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcrypt(t *testing.T) {
	pwd := []byte("mypassword")
	out, err := execute(t, bcryptCmd, pwd)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(out, pwd); err != nil {
		t.Fatalf("error checking hash %v for password \"%s\": %v", out, string(pwd), err)
	}
}

func TestVerifyBcrypt(t *testing.T) {
	hash := []byte("$2a$14$DbrOnW56KeFq3fNtqvHM8epZUPnmIeYlZc/ZlQ/Kvy6Ca.tBAGf9e")
	_, err := execute(t, bcryptVerifyCmd, hash, "--password", "mypassword")
	if err != nil {
		t.Fatal(err)
	}
}
