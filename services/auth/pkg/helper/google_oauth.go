package helper

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	GoogleOAuthStateCookie = "ugen_google_oauth_state"
	GoogleAccessCookie     = "access_token"
	GoogleRefreshCookie    = "refresh_token"
	googleAuthURL          = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL         = "https://oauth2.googleapis.com/token"
	googleUserInfoURL      = "https://www.googleapis.com/oauth2/v3/userinfo"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
	Secret       string
}

type GoogleOAuthState struct {
	Nonce string `json:"nonce"`
	Exp   int64  `json:"exp"`
}

type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type GoogleProfile struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

func NewGoogleOAuthState(secret string, ttl time.Duration) (state, nonce string, err error) {
	rawNonce := make([]byte, 32)
	if _, err = rand.Read(rawNonce); err != nil {
		return "", "", err
	}
	nonce = base64.RawURLEncoding.EncodeToString(rawNonce)

	payload, err := json.Marshal(GoogleOAuthState{
		Nonce: nonce,
		Exp:   time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := signGoogleOAuthState(encodedPayload, secret)

	return encodedPayload + "." + signature, nonce, nil
}

func ValidateGoogleOAuthState(state, cookieNonce, secret string) error {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return errors.New("invalid google oauth state")
	}

	expectedSig := signGoogleOAuthState(parts[0], secret)
	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return errors.New("invalid google oauth state signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}

	var parsed GoogleOAuthState
	if err = json.Unmarshal(payload, &parsed); err != nil {
		return err
	}

	if parsed.Exp < time.Now().Unix() {
		return errors.New("google oauth state expired")
	}
	if parsed.Nonce == "" || parsed.Nonce != cookieNonce {
		return errors.New("google oauth state nonce mismatch")
	}

	return nil
}

func GoogleAuthRedirectURL(cfg GoogleOAuthConfig, state string) (string, error) {
	if cfg.ClientID == "" || cfg.CallbackURL == "" {
		return "", errors.New("google oauth client id and callback url are required")
	}

	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("redirect_uri", cfg.CallbackURL)
	values.Set("response_type", "code")
	values.Set("scope", "openid email profile")
	values.Set("state", state)
	values.Set("prompt", "select_account")
	values.Set("access_type", "online")

	return googleAuthURL + "?" + values.Encode(), nil
}

func ExchangeGoogleCode(ctx context.Context, httpClient *http.Client, cfg GoogleOAuthConfig, code string) (*GoogleTokenResponse, error) {
	if code == "" {
		return nil, errors.New("google auth code is required")
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.CallbackURL == "" {
		return nil, errors.New("google oauth config is incomplete")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", cfg.CallbackURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp GoogleTokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || tokenResp.Error != "" {
		if tokenResp.Description != "" {
			return nil, errors.New(tokenResp.Description)
		}
		if tokenResp.Error != "" {
			return nil, errors.New(tokenResp.Error)
		}
		return nil, fmt.Errorf("google token exchange failed with status %d", resp.StatusCode)
	}
	if tokenResp.AccessToken == "" {
		return nil, errors.New("google access token is empty")
	}

	return &tokenResp, nil
}

func FetchGoogleProfile(ctx context.Context, httpClient *http.Client, accessToken string) (*GoogleProfile, error) {
	if accessToken == "" {
		return nil, errors.New("google access token is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google userinfo failed with status %d", resp.StatusCode)
	}

	var raw struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		Error         string `json:"error"`
	}
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.Error != "" {
		return nil, errors.New(raw.Error)
	}

	profile := &GoogleProfile{
		ID:      raw.Sub,
		Email:   strings.ToLower(strings.TrimSpace(raw.Email)),
		Name:    strings.TrimSpace(raw.Name),
		Picture: raw.Picture,
	}
	switch v := raw.EmailVerified.(type) {
	case bool:
		profile.EmailVerified = v
	case string:
		profile.EmailVerified = strings.EqualFold(v, "true")
	}

	if profile.ID == "" || profile.Email == "" || !profile.EmailVerified {
		return nil, errors.New("google email is not verified")
	}

	return profile, nil
}

func signGoogleOAuthState(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
