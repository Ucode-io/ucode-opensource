package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func createYandexMetricCounter(ctx context.Context, token, name, site string) (int64, error) {
	if token == "" {
		return 0, nil
	}

	body, err := json.Marshal(map[string]any{
		"counter": map[string]any{
			"name": name,
			"site": site,
			"code_options": map[string]any{
				"async":       1,
				"visor":       1,
				"track_hash":  1,
				"clickmap":    1,
				"in_one_line": 1,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.YandexMetricCountersURL, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "OAuth "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("yandex api call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("yandex api %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Counter struct {
			ID int64 `json:"id"`
		} `json:"counter"`
	}
	if err = json.Unmarshal(respBytes, &result); err != nil {
		return 0, fmt.Errorf("parse response: %w", err)
	}

	return result.Counter.ID, nil
}

func (p *ChatProcessor) injectYandexMetrica(ctx context.Context, projectName, mfeURL string, files []models.ProjectFile) {
	if p.baseConf.YandexMetricToken == "" {
		return
	}

	yandexID, err := createYandexMetricCounter(ctx, p.baseConf.YandexMetricToken, slugify(projectName), mfeURL)
	if err != nil {
		log.Printf("[yandex-metrica] counter creation failed (non-fatal): %v", err)
		return
	}
	if yandexID == 0 {
		return
	}

	log.Printf("[yandex-metrica] counter created: id=%d site=%s", yandexID, mfeURL)

	updatedFiles := appendYandexMetricaID(files, yandexID)
	var envFiles []models.ProjectFile
	for _, f := range updatedFiles {
		if f.Path == ".env" || f.Path == ".env.development" || f.Path == ".env.production" {
			envFiles = append(envFiles, f)
		}
	}
	if len(envFiles) == 0 {
		return
	}

	if pushErr := p.pushMicrofrontendChangesChunked(ctx, envFiles); pushErr != nil {
		log.Printf("[yandex-metrica] .env push failed (non-fatal): %v", pushErr)
	}
}

func appendYandexMetricaID(files []models.ProjectFile, id int64) []models.ProjectFile {
	line := fmt.Sprintf("VITE_YANDEX_METRICA_ID=%d\n", id)
	envTargets := map[string]bool{".env": true, ".env.development": true, ".env.production": true}
	for i, f := range files {
		if envTargets[f.Path] {
			delete(envTargets, f.Path)
			if !strings.Contains(f.Content, "VITE_YANDEX_METRICA_ID") {
				files[i].Content += line
			}
		}
	}
	for path := range envTargets {
		files = append(files, models.ProjectFile{Path: path, Content: line})
	}
	return files
}
