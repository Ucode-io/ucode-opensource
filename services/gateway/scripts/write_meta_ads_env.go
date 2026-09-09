package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fatal(errors.New("usage: write_meta_ads_env <output-path>"))
	}

	accessToken := requiredEnv("META_ACCESS_TOKEN")
	facebookAppSecret := requiredEnv("UCODE_FACEBOOK_APP_SECRET")
	facebookWebhookVerifyToken := requiredEnv("UCODE_FACEBOOK_WEBHOOK_VERIFY_TOKEN")
	instagramClientSecret := requiredEnv("UCODE_INSTAGRAM_CLIENT_SECRET")
	instagramWebhookVerifyToken := requiredEnv("UCODE_INSTAGRAM_WEBHOOK_VERIFY_TOKEN")

	contents := strings.Join([]string{
		"META_ACCESS_TOKEN=" + accessToken,
		"META_AD_ACCOUNT_ID=843587364827107",
		"META_GRAPH_BASE_URL=https://graph.facebook.com",
		"META_GRAPH_VERSION=v26.0",
		"META_ADS_CACHE_TTL_SEC=86400",
		"META_ADS_REQUEST_TIMEOUT_SEC=30",
		"META_ADS_MAX_RANGE_DAYS=366",
		"META_LEAD_ACTION_TYPES=lead",
		"META_ATTRIBUTION_WINDOWS=",
		"UCODE_FACEBOOK_APP_ID=1393190606241332",
		"UCODE_FACEBOOK_APP_SECRET=" + facebookAppSecret,
		"UCODE_FACEBOOK_REDIRECT_URI=https://api.admin.u-code.io/v1/facebook/callback",
		"UCODE_FACEBOOK_WEBHOOK_VERIFY_TOKEN=" + facebookWebhookVerifyToken,
		"UCODE_INSTAGRAM_CLIENT_ID=1015893261210423",
		"UCODE_INSTAGRAM_CLIENT_SECRET=" + instagramClientSecret,
		"UCODE_INSTAGRAM_REDIRECT_URI=https://api.admin.u-code.io/v1/instagram/callback",
		"UCODE_INSTAGRAM_FRONTEND_SUCCESS_URL=https://crm.ucode.co/settings/integrations",
		"UCODE_INSTAGRAM_FRONTEND_ERROR_URL=https://crm.ucode.co/settings/integrations",
		"UCODE_INSTAGRAM_WEBHOOK_VERIFY_TOKEN=" + instagramWebhookVerifyToken,
		"",
	}, "\n")

	if err := os.WriteFile(os.Args[1], []byte(contents), 0o400); err != nil {
		fatal(err)
	}
	fmt.Println("Backend-only Meta Ads environment created.")
}

func requiredEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		fatal(fmt.Errorf("%s is required", key))
	}
	if strings.ContainsAny(value, "\r\n") {
		fatal(fmt.Errorf("%s contains an invalid line break", key))
	}
	return value
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
