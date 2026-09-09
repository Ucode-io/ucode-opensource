package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/stretchr/testify/require"
)

func TestFacebookMarketingAPICallsUseGrantedUserToken(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		require.Equal(t, "review-user-token", r.URL.Query().Get("access_token"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v26.0/me/businesses":
			require.Equal(t, "id,name", r.URL.Query().Get("fields"))
			_, _ = w.Write([]byte(`{"data":[{"id":"business-1","name":"UCODE"}]}`))
		case "/v26.0/me/adaccounts":
			require.Equal(t, "id,name,account_status", r.URL.Query().Get("fields"))
			_, _ = w.Write([]byte(`{"data":[{"id":"act_123","name":"UCODE Ads"}]}`))
		case "/v26.0/act_123/insights":
			require.Equal(t, "last_30d", r.URL.Query().Get("date_preset"))
			require.Equal(t, "account", r.URL.Query().Get("level"))
			_, _ = w.Write([]byte(`{"data":[{"spend":"10.00","impressions":"100"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	h := HandlerV1{baseConf: config.BaseConfig{
		FacebookGraphBaseURL:    server.URL,
		FacebookGraphAPIVersion: "v26.0",
	}}

	var businesses struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, h.facebookGraphGet(context.Background(), "me/businesses", url.Values{
		"fields": {"id,name"}, "access_token": {"review-user-token"},
	}, &businesses))
	var accounts struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, h.facebookGraphGet(context.Background(), "me/adaccounts", url.Values{
		"fields": {"id,name,account_status"}, "access_token": {"review-user-token"},
	}, &accounts))
	var insights struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, h.facebookGraphGet(context.Background(), accounts.Data[0].ID+"/insights", url.Values{
		"date_preset": {"last_30d"}, "level": {"account"}, "access_token": {"review-user-token"},
	}, &insights))

	require.Equal(t, []string{"/v26.0/me/businesses", "/v26.0/me/adaccounts", "/v26.0/act_123/insights"}, paths)
}

func TestFacebookRunAppReviewCalls(t *testing.T) {
	insightCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "review-user-token", r.URL.Query().Get("access_token"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v26.0/me/adaccounts":
			_, _ = w.Write([]byte(`{"data":[{"id":"act_123"}]}`))
		case "/v26.0/act_123/insights":
			insightCalls++
			require.Equal(t, "spend,impressions", r.URL.Query().Get("fields"))
			require.Equal(t, "last_7d", r.URL.Query().Get("date_preset"))
			_, _ = w.Write([]byte(`{"data":[{"spend":"10.00","impressions":"100"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	h := HandlerV1{baseConf: config.BaseConfig{
		FacebookGraphBaseURL:    server.URL,
		FacebookGraphAPIVersion: "v26.0",
	}}

	successes, attempts, err := h.facebookRunAppReviewCalls(context.Background(), "review-user-token", 3, 3, 0)
	require.NoError(t, err)
	require.Equal(t, 3, successes)
	require.Equal(t, 3, attempts)
	require.Equal(t, 3, insightCalls)
}

func TestFacebookDebugTokenFallsBackToLegacyApp(t *testing.T) {
	var appTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appToken := r.URL.Query().Get("access_token")
		appTokens = append(appTokens, appToken)
		w.Header().Set("Content-Type", "application/json")
		if appToken == "legacy-app|legacy-secret" {
			_, _ = w.Write([]byte(`{"data":{"app_id":"legacy-app","is_valid":true,"expires_at":1893456000}}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"The App_id in the input_token did not match the Viewing App","type":"OAuthException","code":100}}`))
	}))
	defer server.Close()

	h := HandlerV1{baseConf: config.BaseConfig{
		FacebookAppID:           "ucode-app",
		FacebookAppSecret:       "ucode-secret",
		FacebookLegacyAppID:     "legacy-app",
		FacebookLegacyAppSecret: "legacy-secret",
		FacebookGraphBaseURL:    server.URL,
		FacebookGraphAPIVersion: "v26.0",
	}}

	debug, err := h.facebookDebugToken(context.Background(), "legacy-user-token")
	require.NoError(t, err)
	require.True(t, debug.IsValid)
	require.Equal(t, []string{"ucode-app|ucode-secret", "legacy-app|legacy-secret"}, appTokens)
}

func TestFacebookDebugTokenUsesPrimaryAppFirst(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"app_id":"ucode-app","is_valid":true,"expires_at":1893456000}}`))
	}))
	defer server.Close()

	h := HandlerV1{baseConf: config.BaseConfig{
		FacebookAppID:           "ucode-app",
		FacebookAppSecret:       "ucode-secret",
		FacebookLegacyAppID:     "legacy-app",
		FacebookLegacyAppSecret: "legacy-secret",
		FacebookGraphBaseURL:    server.URL,
		FacebookGraphAPIVersion: "v26.0",
	}}

	debug, err := h.facebookDebugToken(context.Background(), "ucode-user-token")
	require.NoError(t, err)
	require.True(t, debug.IsValid)
	require.Equal(t, 1, requests)
}
