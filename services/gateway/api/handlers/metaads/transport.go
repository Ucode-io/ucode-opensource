package metaads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type graphClient struct {
	baseURL    string
	version    string
	accountID  string
	token      string
	http       *http.Client
	maxRetries int
}

func newGraphClient(baseURL, version, accountID, token string, timeout time.Duration) *graphClient {
	return &graphClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		version:    strings.Trim(version, "/"),
		accountID:  strings.TrimPrefix(accountID, "act_"),
		token:      token,
		http:       &http.Client{Timeout: timeout},
		maxRetries: 3,
	}
}

func (c *graphClient) endpoint(path string) string {
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		return path
	}
	return fmt.Sprintf("%s/%s/%s", c.baseURL, c.version, strings.TrimPrefix(path, "/"))
}

func (c *graphClient) request(ctx context.Context, method, path string, values url.Values, out any) error {
	endpoint := c.endpoint(path)
	if method == http.MethodGet && len(values) > 0 {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("parse meta endpoint: %w", err)
		}
		query := parsed.Query()
		for key, items := range values {
			for _, item := range items {
				query.Add(key, item)
			}
		}
		query.Del("access_token")
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
	}

	for attempt := 0; ; attempt++ {
		var body io.Reader
		if method == http.MethodPost {
			body = strings.NewReader(values.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
		if err != nil {
			return fmt.Errorf("build meta request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < c.maxRetries && ctx.Err() == nil {
				if err := waitRetry(ctx, attempt, ""); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("call meta graph api: %w", err)
		}

		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read meta response: %w", readErr)
		}
		if len(responseBody) > maxResponseBytes {
			return fmt.Errorf("meta response exceeds %d bytes", maxResponseBytes)
		}

		if resp.StatusCode >= http.StatusBadRequest {
			apiErr := parseGraphError(resp.StatusCode, responseBody)
			if c.token != "" {
				apiErr.Message = strings.ReplaceAll(apiErr.Message, c.token, "[REDACTED]")
			}
			if attempt < c.maxRetries && apiErr.retryable() {
				if err := waitRetry(ctx, attempt, resp.Header.Get("Retry-After")); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(responseBody, out); err != nil {
			return fmt.Errorf("decode meta response: %w", err)
		}
		return nil
	}
}

func parseGraphError(statusCode int, body []byte) *graphError {
	var response struct {
		Error struct {
			Message      string `json:"message"`
			Type         string `json:"type"`
			Code         int    `json:"code"`
			ErrorSubcode int    `json:"error_subcode"`
			IsTransient  bool   `json:"is_transient"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &response)
	message := response.Error.Message
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return &graphError{
		StatusCode: statusCode,
		Code:       response.Error.Code,
		Subcode:    response.Error.ErrorSubcode,
		Type:       response.Error.Type,
		Message:    message,
		Transient:  response.Error.IsTransient,
	}
}

func waitRetry(ctx context.Context, attempt int, retryAfter string) error {
	delay := 250 * time.Millisecond * time.Duration(1<<attempt)
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
		delay = time.Duration(seconds) * time.Second
	}
	delay += time.Duration(time.Now().UnixNano() % int64(100*time.Millisecond))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type graphPage[T any] struct {
	Data   []T `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

func fetchAll[T any](ctx context.Context, client *graphClient, path string, query url.Values) ([]T, error) {
	items := make([]T, 0)
	next := path
	values := query
	for pageNumber := 0; pageNumber < maxGraphPages; pageNumber++ {
		var page graphPage[T]
		if err := client.request(ctx, http.MethodGet, next, values, &page); err != nil {
			return nil, err
		}
		items = append(items, page.Data...)
		if page.Paging.Next == "" {
			return items, nil
		}
		parsed, err := url.Parse(page.Paging.Next)
		if err != nil {
			return nil, fmt.Errorf("parse meta pagination URL: %w", err)
		}
		base, err := url.Parse(client.baseURL)
		if err != nil || !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host) {
			return nil, fmt.Errorf("meta pagination URL points to an unexpected host")
		}
		pageQuery := parsed.Query()
		pageQuery.Del("access_token")
		parsed.RawQuery = pageQuery.Encode()
		next = parsed.String()
		values = nil
	}
	return nil, fmt.Errorf("meta pagination exceeded %d pages", maxGraphPages)
}
