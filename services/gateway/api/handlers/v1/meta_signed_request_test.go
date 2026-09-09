package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMetaSignedRequest(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"algorithm":"HMAC-SHA256","user_id":"17841400000000000"}`))
	value := signedMetaRequest("legacy-secret", payload)

	decoded, err := parseMetaSignedRequest(value, "primary-secret", "legacy-secret")
	require.NoError(t, err)
	require.Equal(t, "17841400000000000", decoded.UserID)

	_, err = parseMetaSignedRequest(value, "primary-secret")
	require.Error(t, err)
	_, err = parseMetaSignedRequest(value+"tampered", "legacy-secret")
	require.Error(t, err)
}

func TestParseMetaSignedRequestRejectsUnsupportedAlgorithm(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"algorithm":"none","user_id":"17841400000000000"}`))
	_, err := parseMetaSignedRequest(signedMetaRequest("secret", payload), "secret")
	require.Error(t, err)
}

func signedMetaRequest(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + payload
}
