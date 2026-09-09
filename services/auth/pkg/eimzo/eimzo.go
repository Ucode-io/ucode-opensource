package eimzo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
)

func GetChallenge(cfg config.BaseConfig) (*pb.GetChallengeResponse, error) {
	url := fmt.Sprintf("%s/frontend/challenge", cfg.EImzoBaseUrl)

	resp, err := doRequest[pb.GetChallengeResponse](cfg, url, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ExtractUserFromPKCS7(cfg config.BaseConfig, pkcs7, clientIp string) (*models.ExtractUserFromPKCS7Response, error) {
	url := fmt.Sprintf("%s/backend/auth", cfg.EImzoBaseUrl)

	payload := []byte(pkcs7)

	headers := map[string]string{
		"X-Real-IP": clientIp,
		"Host":      cfg.EImzoHost,
	}

	resp, err := doRequest[models.ExtractUserFromPKCS7Response](cfg, url, http.MethodPost, payload, headers)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func doRequest[T any](cfg config.BaseConfig, url, method string, payload []byte, headers map[string]string) (*T, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}

	req.SetBasicAuth(cfg.EImzoUsername, cfg.EImzoPassword)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var response T
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return &response, nil
}

func ExtractFromX500(x500name, key string) string {
	pattern := fmt.Sprintf(`%s=([^,]+)`, regexp.QuoteMeta(key))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(x500name)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}
