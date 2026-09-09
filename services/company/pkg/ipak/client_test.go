package ipak

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// captureServer records the last request the client made so tests can assert on
// the JSON-RPC envelope, auth header and params.
type captured struct {
	authHeader string
	method     string
	params     map[string]interface{}
}

func newServer(t *testing.T, cap *captured, respBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.authHeader = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var env struct {
			Method string                 `json:"method"`
			Params map[string]interface{} `json:"params"`
		}
		_ = json.Unmarshal(body, &env)
		cap.method = env.Method
		cap.params = env.Params
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
}

func TestCreateTokenSendsSomAmountAndBearer(t *testing.T) {
	cap := &captured{}
	srv := newServer(t, cap, `{"result":{"code":0,"transfer_id":"tid-1","payment_url":"https://bank/pay/tid-1"}}`)
	defer srv.Close()

	c := New(srv.URL, "testtoken", "", "", false, time.Second)
	res, err := c.CreateToken(context.Background(), CreateTokenParams{
		OrderID:     "order-1",
		Amount:      150000, // 150 000 UZS
		Description: "top-up",
		SuccessURL:  "https://app/success",
		FailURL:     "https://app/fail",
	})
	if err != nil {
		t.Fatalf("CreateToken error: %v", err)
	}
	if res.TransferID != "tid-1" || res.PaymentURL != "https://bank/pay/tid-1" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if cap.authHeader != "Bearer testtoken" {
		t.Fatalf("expected Bearer auth, got %q", cap.authHeader)
	}
	if cap.method != "transfer.create_token" {
		t.Fatalf("expected method transfer.create_token, got %q", cap.method)
	}
	// Amount must be sent in som (150000), NOT tiyin (15000000).
	if got, ok := cap.params["amount"].(float64); !ok || got != 150000 {
		t.Fatalf("expected amount 150000 (som), got %v", cap.params["amount"])
	}
}

func TestCreateTokenRejectsNonZeroCode(t *testing.T) {
	cap := &captured{}
	srv := newServer(t, cap, `{"result":{"code":2,"transfer_id":"","payment_url":""}}`)
	defer srv.Close()

	c := New(srv.URL, "tok", "", "", false, time.Second)
	if _, err := c.CreateToken(context.Background(), CreateTokenParams{OrderID: "o", Amount: 1000, Description: "d"}); err == nil {
		t.Fatal("expected error on non-zero result code")
	}
}

func TestTransferGetDecodesStatus(t *testing.T) {
	cap := &captured{}
	srv := newServer(t, cap, `{"result":{"id":"tid","status":"success","orderId":"o","amount":1000,"netAmount":1000}}`)
	defer srv.Close()

	c := New(srv.URL, "tok", "", "", false, time.Second)
	res, err := c.TransferGet(context.Background(), "tid")
	if err != nil {
		t.Fatalf("TransferGet error: %v", err)
	}
	if res.Status != "success" || res.NetAmount != 1000 {
		t.Fatalf("unexpected transfer: %+v", res)
	}
	if cap.method != "transfer.get" {
		t.Fatalf("expected method transfer.get, got %q", cap.method)
	}
}

func TestRpcErrorSurfaces(t *testing.T) {
	cap := &captured{}
	srv := newServer(t, cap, `{"error":{"code":401,"message":"unauthorized"}}`)
	defer srv.Close()

	c := New(srv.URL, "tok", "", "", false, time.Second)
	if _, err := c.TransferGet(context.Background(), "tid"); err == nil {
		t.Fatal("expected rpc error to surface")
	}
}

func TestNotConfigured(t *testing.T) {
	c := New("", "", "", "", false, time.Second)
	if c.Configured() {
		t.Fatal("expected Configured() to be false with empty base url/token")
	}
	if _, err := c.CreateToken(context.Background(), CreateTokenParams{OrderID: "o", Amount: 1, Description: "d"}); err == nil {
		t.Fatal("expected error when client not configured")
	}
}

func TestProxyRoutesRequest(t *testing.T) {
	var (
		proxied   bool
		gotMethod string
	)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		body, _ := io.ReadAll(r.Body)
		var env struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &env)
		gotMethod = env.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"id":"tid","status":"success","orderId":"o","amount":1,"netAmount":1}}`))
	}))
	defer proxy.Close()

	// http:// target so the proxy receives the full forwarded request. The host is
	// unroutable, so the call can only succeed via the proxy — proving traffic is
	// tunnelled through it (as it will be through the UZ proxy in production).
	c := New("http://bank.invalid/api", "tok", proxy.URL, "", false, time.Second)
	res, err := c.TransferGet(context.Background(), "tid")
	if err != nil {
		t.Fatalf("TransferGet via proxy error: %v", err)
	}
	if !proxied {
		t.Fatal("expected request to be routed through the proxy")
	}
	if gotMethod != "transfer.get" {
		t.Fatalf("proxy saw method %q, want transfer.get", gotMethod)
	}
	if res.Status != "success" {
		t.Fatalf("unexpected result via proxy: %+v", res)
	}
}

// The bank serves a cert from a CA outside the default bundle; these two tests
// mirror that with httptest's self-signed server.
func TestTLSSelfSignedFailsThenTrusts(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"id":"tid","status":"success","orderId":"o","amount":1,"netAmount":1}}`))
	}))
	defer srv.Close()

	// Default: self-signed cert is untrusted -> verification error.
	c := New(srv.URL, "tok", "", "", false, time.Second)
	if _, err := c.TransferGet(context.Background(), "tid"); err == nil {
		t.Fatal("expected TLS verification failure against self-signed server")
	}

	// insecureSkip -> connects despite untrusted cert.
	c = New(srv.URL, "tok", "", "", true, time.Second)
	if res, err := c.TransferGet(context.Background(), "tid"); err != nil {
		t.Fatalf("insecureSkip should connect: %v", err)
	} else if res.Status != "success" {
		t.Fatalf("unexpected: %+v", res)
	}

	// Trusting the server's own cert as CA -> connects with verification on.
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	c = New(srv.URL, "tok", "", string(caPEM), false, time.Second)
	if res, err := c.TransferGet(context.Background(), "tid"); err != nil {
		t.Fatalf("CA-trusted client should connect: %v", err)
	} else if res.Status != "success" {
		t.Fatalf("unexpected: %+v", res)
	}
}
