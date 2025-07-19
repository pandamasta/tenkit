package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var secretKey = []byte("replace-this-with-env-secret")

func GenerateSignupToken(email, org string, expires time.Time) (string, error) {
	payload := fmt.Sprintf("%s|%s|%d", email, org, expires.Unix())
	h := hmac.New(sha256.New, secretKey)
	h.Write([]byte(payload))
	sig := h.Sum(nil)
	token := fmt.Sprintf("%s.%s",
		base64.URLEncoding.EncodeToString([]byte(payload)),
		base64.URLEncoding.EncodeToString(sig),
	)
	return token, nil
}

func ValidateSignupToken(token string) (email, org string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", "", false
	}
	payloadBytes, _ := base64.URLEncoding.DecodeString(parts[0])
	sigBytes, _ := base64.URLEncoding.DecodeString(parts[1])

	expected := hmac.New(sha256.New, secretKey)
	expected.Write(payloadBytes)
	if !hmac.Equal(expected.Sum(nil), sigBytes) {
		return "", "", false
	}

	fields := strings.Split(string(payloadBytes), "|")
	if len(fields) != 3 {
		return "", "", false
	}

	email, org = fields[0], fields[1]
	exp, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", "", false
	}
	return email, org, true
}

func GenerateUserToken(email string, tenantID int64, expires time.Time) (string, error) {
	payload := fmt.Sprintf("%s|%d|%d", email, tenantID, expires.Unix())
	h := hmac.New(sha256.New, secretKey)
	h.Write([]byte(payload))
	sig := h.Sum(nil)
	return fmt.Sprintf("%s.%s",
		base64.URLEncoding.EncodeToString([]byte(payload)),
		base64.URLEncoding.EncodeToString(sig),
	), nil
}

func ValidateUserToken(token string) (email string, tenantID int64, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", 0, false
	}
	payloadBytes, _ := base64.URLEncoding.DecodeString(parts[0])
	sigBytes, _ := base64.URLEncoding.DecodeString(parts[1])
	mac := hmac.New(sha256.New, secretKey)
	mac.Write(payloadBytes)
	if !hmac.Equal(mac.Sum(nil), sigBytes) {
		return "", 0, false
	}

	fields := strings.Split(string(payloadBytes), "|")
	if len(fields) != 3 {
		return "", 0, false
	}
	email = fields[0]
	id, err := strconv.ParseInt(fields[1], 10, 64)
	exp, err2 := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || err2 != nil || time.Now().Unix() > exp {
		return "", 0, false
	}
	return email, id, true
}
