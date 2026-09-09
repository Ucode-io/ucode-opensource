package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

func verifyMetaWebhookSignature(header, prefix string, body []byte, secrets ...string) bool {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, prefix) {
		return false
	}

	provided, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}

	matched := 0
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		matched |= subtle.ConstantTimeCompare(provided, mac.Sum(nil))
	}
	return matched == 1
}
