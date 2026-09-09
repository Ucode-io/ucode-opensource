package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyMetaWebhookSignature(t *testing.T) {
	body := []byte(`{"object":"page"}`)
	header := signedMetaWebhookHeader("legacy-secret", body)

	require.True(t, verifyMetaWebhookSignature(header, "sha256=", body, "primary-secret", "legacy-secret"))
	require.False(t, verifyMetaWebhookSignature(header, "sha256=", body, "primary-secret"))
	require.False(t, verifyMetaWebhookSignature("sha1=invalid", "sha256=", body, "legacy-secret"))
}

func signedMetaWebhookHeader(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
