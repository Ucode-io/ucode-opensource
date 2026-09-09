package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/gin-gonic/gin"
)

func TestGenerateKpProposalReturnsPrototype(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body kpAgentRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode agent request: %v", err)
		}
		if body.PrototypePublicBaseURL != "https://api.admin.u-code.io/v1/kp-prototypes" {
			t.Fatalf("unexpected prototype base URL: %q", body.PrototypePublicBaseURL)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"ok":true,
			"requestId":"KP-1",
			"title":"KP",
			"html":"<!doctype html><title>KP</title>",
			"pageCount":9,
			"prototypeUrl":"https://api.admin.u-code.io/v1/kp-prototypes/public-id/",
			"prototype":{"url":"https://api.admin.u-code.io/v1/kp-prototypes/public-id/","qaStatus":"PASS","screenCount":55,"rendererVersion":"app-prototype-v1"}
		}`))
	}))
	defer agent.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "http://gateway.internal/v1/kp-proposals", strings.NewReader(`{"prompt":"Build CRM"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("X-Forwarded-Proto", "https")
	context.Request.Header.Set("X-Forwarded-Host", "api.admin.u-code.io")
	context.Set("project_id", "11111111-1111-4111-8111-111111111111")
	context.Set("environment_id", "22222222-2222-4222-8222-222222222222")

	handler := HandlerV1{baseConf: config.BaseConfig{KpAgentURL: agent.URL}}
	handler.GenerateKpProposal(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			PrototypeURL string `json:"prototypeUrl"`
			Prototype    struct {
				URL         string `json:"url"`
				ScreenCount int    `json:"screenCount"`
			} `json:"prototype"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode gateway response: %v", err)
	}
	if envelope.Data.PrototypeURL == "" || envelope.Data.Prototype.URL != envelope.Data.PrototypeURL {
		t.Fatalf("prototype URL missing from response: %s", recorder.Body.String())
	}
	if envelope.Data.Prototype.ScreenCount != 55 {
		t.Fatalf("prototype metadata missing from response: %s", recorder.Body.String())
	}
}

func TestGetKpPrototypeProxiesAgentHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/p/public-id/" {
			t.Fatalf("unexpected agent path: %s", request.URL.Path)
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Content-Security-Policy", "default-src 'none'")
		_, _ = response.Write([]byte("<!doctype html><title>Prototype</title>"))
	}))
	defer agent.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/v1/kp-prototypes/public-id/", nil)
	context.Params = gin.Params{{Key: "publicId", Value: "public-id"}}

	handler := HandlerV1{baseConf: config.BaseConfig{KpAgentURL: agent.URL}}
	handler.GetKpPrototype(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Prototype") {
		t.Fatalf("prototype HTML missing: %s", recorder.Body.String())
	}
	if recorder.Header().Get("Content-Security-Policy") != "default-src 'none'" {
		t.Fatalf("prototype CSP was not preserved")
	}
}

func TestIsValidKpPrototypeID(t *testing.T) {
	if !isValidKpPrototypeID("aq-oG7w1dq-I") {
		t.Fatal("expected generated public ID to be accepted")
	}
	for _, value := range []string{"short", "../private", "bad/id", "bad id"} {
		if isValidKpPrototypeID(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
