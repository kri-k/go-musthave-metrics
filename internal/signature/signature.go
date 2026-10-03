package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const Header = "HashSHA256"

func digest(body []byte, key string) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(body)
	return h.Sum(nil)
}

func Sign(body []byte, key string) string {
	return hex.EncodeToString(digest(body, key))
}

func Verify(body []byte, key, signature string) bool {
	got, err := hex.DecodeString(signature)
	return err == nil && hmac.Equal(got, digest(body, key))
}
