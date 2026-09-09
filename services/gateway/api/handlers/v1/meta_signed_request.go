package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type metaSignedRequestPayload struct {
	Algorithm string `json:"algorithm"`
	UserID    string `json:"user_id"`
}

func parseMetaSignedRequest(value string, secrets ...string) (metaSignedRequestPayload, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return metaSignedRequestPayload{}, errors.New("invalid signed_request format")
	}

	provided, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return metaSignedRequestPayload{}, errors.New("invalid signed_request signature")
	}

	matched := 0
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(parts[1]))
		matched |= subtle.ConstantTimeCompare(provided, mac.Sum(nil))
	}
	if matched != 1 {
		return metaSignedRequestPayload{}, errors.New("signed_request verification failed")
	}

	payloadBody, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return metaSignedRequestPayload{}, errors.New("invalid signed_request payload")
	}
	var payload metaSignedRequestPayload
	if err = json.Unmarshal(payloadBody, &payload); err != nil {
		return metaSignedRequestPayload{}, errors.New("invalid signed_request payload")
	}
	if !strings.EqualFold(payload.Algorithm, "HMAC-SHA256") || strings.TrimSpace(payload.UserID) == "" {
		return metaSignedRequestPayload{}, errors.New("unsupported signed_request payload")
	}
	return payload, nil
}
