package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/TylerBrock/colorjson"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

func RandomBytes(length int) []byte {
	b := make([]byte, length)
	rand.Read(b)
	return b
}

func BcryptHash(value []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(value, 14)
}

func BcryptCompare(value []byte, hash []byte) bool {
	err := bcrypt.CompareHashAndPassword(hash, value)
	return err == nil
}

func Sha256Hash(value []byte) []byte {
	h := sha256.New()
	h.Write(value)
	return h.Sum(nil)
}

func Sha256Compare(value []byte, hash []byte) bool {
	h := Sha256Hash(value)
	return bytes.Equal(hash, h)
}

func Encrypt(value []byte, passPhrase []byte) ([]byte, error) {
	block, err := aes.NewCipher(passPhrase)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	return aesGCM.Seal(nonce, nonce, value, nil), nil
}

func EncryptWithHash(value []byte, passPhrase []byte) ([]byte, []byte, error) {
	hash := Sha256Hash(value)
	enc, err := Encrypt(value, passPhrase)
	if err != nil {
		return nil, nil, err
	}
	return enc, hash, nil
}

func Decrypt(value []byte, passPhrase []byte) ([]byte, error) {
	block, err := aes.NewCipher(passPhrase)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aesGCM.NonceSize()
	nonce, secValue := value[:nonceSize], value[nonceSize:]
	return aesGCM.Open(nil, nonce, secValue, nil)
}

func DecryptAndVerify(value []byte, passPhrase []byte, hash []byte) ([]byte, error) {
	dec, err := Decrypt(value, passPhrase)
	if err != nil {
		return nil, err
	}
	if !Sha256Compare(dec, hash) {
		return nil, fmt.Errorf("Decrypted value not match hash")
	}
	return dec, nil
}

type JWTParams struct {
	Scope     string
	Subject   string
	Issuer    string
	Secret    []byte
	ExpiresAt time.Time
}

type JWTInfo struct {
	Subject   string
	Issuer    string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

func CreateJWT(params JWTParams) (string, error) {
	claims := jwt.StandardClaims{
		ExpiresAt: params.ExpiresAt.Unix(),
		IssuedAt:  time.Now().Unix(),
		Issuer:    params.Issuer,
		Subject:   params.Subject,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(params.Secret)
}

func ValidateJWT(jwtString string, issuer string, secret []byte) error {
	token, err := jwt.ParseWithClaims(jwtString, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("Invalid JWT")
	}
	claims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		return fmt.Errorf("Cannot obtain JWT claims")
	}
	if issuer != "" && claims.Issuer != issuer {
		return fmt.Errorf(`JWT issuer "%s" not match "%s"`, claims.Issuer, issuer)
	}
	return nil
}

func ParseJWT(jwtString string, params JWTParams) (*jwt.StandardClaims, error) {
	token, err := jwt.ParseWithClaims(jwtString, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return params.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("Invalid JWT")
	}
	claims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		return nil, fmt.Errorf("Cannot obtain JWT claims")
	}
	return claims, err
}

func ExtractInfoFromJWT(jwtString string) (*jwt.StandardClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(jwtString, &jwt.StandardClaims{})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok {
		return nil, fmt.Errorf("Cannot obtain JWT claims")
	}
	return claims, nil
}

func NetworkContainsIP(network string, ip string) (bool, error) {
	_, ipv4Net, err := net.ParseCIDR(network)
	if err != nil {
		return false, err
	}
	ipo := net.ParseIP(ip)
	return ipv4Net.Contains(ipo), nil
}

func Base64Encode(value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}

func Base64URLEncode(value []byte) string {
	return base64.URLEncoding.EncodeToString(value)
}

func Base64Decode(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func Base64URLDecode(value string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(value)
}

func Base64NoPaddingDecode(value string) ([]byte, error) {
	return base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(value)
}

func Base64URLNoPaddingDecode(value string) ([]byte, error) {
	return base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(value)
}

func HexEncode(value []byte) string {
	return hex.EncodeToString(value)
}

func HexDecode(value string) ([]byte, error) {
	return hex.DecodeString(value)
}

func URLEncode(value string) string {
	return url.QueryEscape(value)
}

func URLDecode(value string) (string, error) {
	return url.QueryUnescape(value)
}

func PrettifyJSON(str string) ([]byte, error) {
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(str), &obj)
	if err != nil {
		return []byte{}, err
	}
	return GetPrettyJSON(obj)
}

func GetPrettyJSON(v interface{}) ([]byte, error) {
	f := colorjson.NewFormatter()
	f.Indent = 2
	return f.Marshal(v)
}
