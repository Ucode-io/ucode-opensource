package v1

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"

	"github.com/spf13/cast"
)

const (
	webFetchTimeout      = 20 * time.Second
	webFetchDialTimeout  = 10 * time.Second
	webFetchMaxBodyBytes = 5 * 1024 * 1024 // cap the body handed back to the model at 5 MB
	webFetchMaxRedirects = 5
)

// webFetchTool exposes a guarded HTTP GET to the agent so it can research
// external information (e.g. exchange rates) that is not stored in the project's
// own tables. The model decides when and what to fetch; the server executes it.
func webFetchTool() ai.ToolDef {
	return ai.ToolDef{
		Name:        "web_fetch",
		Description: "Fetch the contents of a public web URL (e.g. a public JSON API or web page) to research up-to-date external information such as exchange rates, prices, or reference data. Returns the response body as text, truncated if large. Use this only when the answer requires data that is not in the application's own database.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Absolute http:// or https:// URL of a public endpoint to fetch.",
				},
			},
			"required": []string{"url"},
		},
	}
}

// executeWebFetch performs the agent's web_fetch call. It validates the URL,
// fetches it through an SSRF-hardened client, and returns the status, content
// type and (size-capped) body as a JSON string. Non-2xx responses are flagged as
// tool errors but still return the body so the model can reason about them.
func executeWebFetch(ctx context.Context, call ai.ToolCall) (string, bool) {
	rawURL := strings.TrimSpace(cast.ToString(call.Input["url"]))
	if rawURL == "" {
		return "error: url is required", true
	}
	// Accept bare domains the model often produces (e.g. "example.com/path") by
	// defaulting to https rather than rejecting them.
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "error: invalid url: " + err.Error(), true
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "error: only http and https URLs are allowed", true
	}
	if parsed.Host == "" {
		return "error: url must include a host", true
	}
	if parsed.User != nil {
		return "error: URLs with embedded credentials are not allowed", true
	}

	reqCtx, cancel := context.WithTimeout(ctx, webFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "error: " + err.Error(), true
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "ucode-agent/1.0")

	resp, err := webFetchHTTPClient.Do(req)
	if err != nil {
		return "error: fetch failed: " + err.Error(), true
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, webFetchMaxBodyBytes+1))
	if err != nil {
		return "error: reading response: " + err.Error(), true
	}
	truncated := false
	if len(body) > webFetchMaxBodyBytes {
		body = body[:webFetchMaxBodyBytes]
		truncated = true
	}

	result := map[string]any{
		"status":       resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"body":         string(body),
	}
	if truncated {
		result["truncated"] = true
	}

	return marshalToolResult(result), resp.StatusCode >= 400
}

// webFetchHTTPClient is the single SSRF-hardened client used for all agent web
// fetches. It blocks dials to non-public addresses at the socket level, so the
// guard also covers redirects and DNS-rebinding.
var webFetchHTTPClient = newWebFetchHTTPClient()

func newWebFetchHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: webFetchDialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("could not resolve %q to an IP", host)
			}
			if isBlockedIP(ip) {
				return fmt.Errorf("address %s is not a public endpoint", ip)
			}
			return nil
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   webFetchDialTimeout,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableKeepAlives:     true,
	}

	return &http.Client{
		Timeout:   webFetchTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= webFetchMaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to disallowed scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
}

// isBlockedIP reports whether an IP is outside the public internet and therefore
// must never be reached by an agent web fetch (SSRF defense): loopback, private,
// link-local (incl. cloud metadata 169.254.169.254), multicast, unspecified, and
// carrier-grade NAT ranges.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 0.0.0.0/8 ("this network") and 100.64.0.0/10 (carrier-grade NAT).
		if ip4[0] == 0 {
			return true
		}
		if ip4[0] == 100 && ip4[1]&0xC0 == 64 {
			return true
		}
	}
	return false
}
