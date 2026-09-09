// Package ipak is a thin client for the Ipak Yo'li Bank E-Comm API
// (https://ecom.ipakyulibank.uz/docs/api/). It is used only for the
// Visa/Mastercard hosted-page ("dynamic link") flow: transfer.create_token to
// open a payment, transfer.get to verify it, transfer.cancel to refund.
//
// Unlike Payme (which authenticates with an "X-auth: merchant:key" header), Ipak
// Yo'li uses JSON-RPC 2.0 with a Bearer access token, so this cannot reuse the
// Payme sendRequest helper.
package ipak

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a single Ipak Yo'li cashbox (one AccessToken per cashbox).
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New builds a client. baseURL is e.g. https://ecom.ipakyulibank.uz/api (prod)
// or the staging equivalent; token is the cashbox AccessToken. proxyURL, when
// non-empty (e.g. http://user:pass@<uz-ip>:8888), tunnels every request through
// that forward proxy so the bank sees a Uzbek-region source IP; the TLS session
// stays end-to-end with the bank (HTTP CONNECT), so cert validation is unaffected.
//
// The bank serves a certificate from a CA that is not in the default trust bundle
// (a local Uzbek CA), so pass its CA chain via caCertPEM to trust it. insecureSkip
// disables verification entirely -- a staging-only escape hatch; never use in prod.
func New(baseURL, token, proxyURL, caCertPEM string, insecureSkip bool, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	// Clone the default transport so we keep its sane keep-alive/timeout defaults
	// and only override what we need. custom stays false when nothing changed.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	custom := false

	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
			custom = true
		}
	}

	switch {
	case insecureSkip:
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		custom = true
	case caCertPEM != "":
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if pool.AppendCertsFromPEM([]byte(caCertPEM)) {
			transport.TLSClientConfig = &tls.Config{RootCAs: pool}
			custom = true
		}
	}

	httpClient := &http.Client{Timeout: timeout}
	if custom {
		httpClient.Transport = transport
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    httpClient,
	}
}

// Configured reports whether the client has both a base URL and a token. Callers
// use this to fail fast with a clear error before the bank enables the cashbox.
func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.token != ""
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("ipak rpc error %d: %s", e.Code, e.Message)
}

// CreateTokenParams are the inputs to transfer.create_token. Amount is in UZS
// (som, not tiyin), matching the bank's documented unit.
type CreateTokenParams struct {
	OrderID          string
	Amount           float64
	Description      string
	SuccessURL       string
	FailURL          string
	ExpiresInMinutes int
	Lang             string
}

type CreateTokenResult struct {
	Code       int    `json:"code"`
	TransferID string `json:"transfer_id"`
	PaymentURL string `json:"payment_url"`
}

// TransferGetResult is the subset of transfer.get we rely on. netAmount is the
// amount actually settled (minus commissions); status is the source of truth for
// whether to credit the customer.
type TransferGetResult struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	OrderID   string  `json:"orderId"`
	Amount    float64 `json:"amount"`
	NetAmount float64 `json:"netAmount"`
}

type TransferCancelResult struct {
	TransferID string `json:"transfer_id"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
}

// CreateToken opens a hosted payment and returns the URL the customer visits.
func (c *Client) CreateToken(ctx context.Context, p CreateTokenParams) (*CreateTokenResult, error) {
	lang := p.Lang
	if lang == "" {
		lang = "ru"
	}
	params := map[string]interface{}{
		"order_id": p.OrderID,
		// som has no minor units; round to an integer to avoid float formatting.
		"amount": int64(math.Round(p.Amount)),
		"details": map[string]interface{}{
			"description": p.Description,
		},
		"lang": lang,
	}
	if p.SuccessURL != "" {
		params["success_url"] = p.SuccessURL
	}
	if p.FailURL != "" {
		params["fail_url"] = p.FailURL
	}
	if p.ExpiresInMinutes > 0 {
		params["expires_in_minutes"] = p.ExpiresInMinutes
	}

	var result CreateTokenResult
	if err := c.call(ctx, "transfer.create_token", params, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("ipak create_token returned code %d", result.Code)
	}
	return &result, nil
}

// TransferGet fetches the current state of a payment by transfer_id.
func (c *Client) TransferGet(ctx context.Context, transferID string) (*TransferGetResult, error) {
	var result TransferGetResult
	if err := c.call(ctx, "transfer.get", map[string]interface{}{"id": transferID}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TransferCancel issues a full refund (success) or releases a hold (held).
func (c *Client) TransferCancel(ctx context.Context, transferID string) (*TransferCancelResult, error) {
	var result TransferCancelResult
	if err := c.call(ctx, "transfer.cancel", map[string]interface{}{"transfer_id": transferID}, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return &result, fmt.Errorf("ipak transfer.cancel returned code %d: %s", result.Code, result.Message)
	}
	return &result, nil
}

// call performs one JSON-RPC 2.0 POST, decoding result into out. It retries a few
// times on HTTP 429 (the endpoint is limited to 5 requests / 10s per IP).
func (c *Client) call(ctx context.Context, method string, params interface{}, out interface{}) error {
	if !c.Configured() {
		return fmt.Errorf("ipak client is not configured (missing base url or access token)")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("%d", time.Now().UnixNano()),
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Methods are dispatched to per-family endpoints under the base URL: transfer.*
	// -> {base}/transfer, tokenized.* -> {base}/tokenized-contract. Posting to the
	// bare base URL returns 404 "Cannot POST /".
	endpoint := strings.TrimRight(c.baseURL, "/")
	switch {
	case strings.HasPrefix(method, "transfer."):
		endpoint += "/transfer"
	case strings.HasPrefix(method, "tokenized."):
		endpoint += "/tokenized-contract"
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Simple linear backoff; the window is 10s so waits stay short.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("ipak rate limited (429)")
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("ipak http %d: %s", resp.StatusCode, string(respBody))
		}

		var envelope struct {
			Result json.RawMessage `json:"result"`
			Error  *rpcError       `json:"error"`
		}
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return fmt.Errorf("ipak decode response from %s (status %d, content-type %q): %w; body=%.300s",
				endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), err, string(respBody))
		}
		if envelope.Error != nil {
			return envelope.Error
		}
		if out != nil && len(envelope.Result) > 0 {
			if err := json.Unmarshal(envelope.Result, out); err != nil {
				return fmt.Errorf("ipak decode result: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("ipak call %s failed after %d attempts: %w", method, maxAttempts, lastErr)
}
