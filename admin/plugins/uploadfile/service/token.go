package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}

func hashPassword(password string) string {
	salt := randomHex(12)
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return salt + "$" + base64.RawURLEncoding.EncodeToString(sum[:])
}

func verifyPassword(encoded string, password string) bool {
	salt, digest, ok := strings.Cut(encoded, "$")
	if !ok || salt == "" || digest == "" {
		return false
	}
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hmac.Equal([]byte(digest), []byte(base64.RawURLEncoding.EncodeToString(sum[:])))
}

func signDownloadToken(secret []byte, shareToken string, passwordHash string, expiresAt time.Time) string {
	payload := shareToken + "|" + strconv.FormatInt(expiresAt.Unix(), 10) + "|" + passwordHash
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func verifyDownloadToken(secret []byte, encoded string, shareToken string, passwordHash string, now time.Time) bool {
	payloadEncoded, sig, ok := strings.Cut(encoded, ".")
	if !ok || payloadEncoded == "" || sig == "" {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return false
	}
	payload := string(payloadBytes)
	expectedPrefix := shareToken + "|"
	if !strings.HasPrefix(payload, expectedPrefix) || !strings.HasSuffix(payload, "|"+passwordHash) {
		return false
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 3 {
		return false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.After(time.Unix(exp, 0)) {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(sig))
}
