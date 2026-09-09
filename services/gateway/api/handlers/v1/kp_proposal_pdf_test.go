package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
)

// kpTestHandler builds a HandlerV1 with a real (quiet) logger — HandleResponse
// logs every non-2xx response via h.log, which panics on the interface's nil
// zero-value, so any test exercising an error path needs this instead of a bare
// HandlerV1{}.
func kpTestHandler(baseConf config.BaseConfig) HandlerV1 {
	return HandlerV1{baseConf: baseConf, log: logger.NewLogger("kp-test", logger.LevelError)}
}

func TestIsValidKpRequestID(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want bool
	}{
		{name: "valid", id: "KP-20260804-D6BE339096AF", want: true},
		{name: "valid with underscore", id: "KP-2026_08_04-abc", want: true},
		{name: "empty", id: "", want: false},
		{name: "missing prefix", id: "20260804-D6BE339096AF", want: false},
		{name: "prefix only, nothing after", id: "KP-", want: false},
		{name: "lowercase prefix rejected", id: "kp-20260804-abc", want: false},
		{name: "contains slash", id: "KP-2026/08/04", want: false},
		{name: "contains space", id: "KP-2026 08 04", want: false},
		{name: "contains path traversal", id: "KP-../../etc/passwd", want: false},
		{name: "over max length", id: "KP-" + strings.Repeat("a", 94), want: false},
		{name: "at max length", id: "KP-" + strings.Repeat("a", 93), want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidKpRequestID(tc.id); got != tc.want {
				t.Fatalf("isValidKpRequestID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

// TestKpProposalCacheEntryRoundTrip guards the wire format shared between
// GenerateKpProposal (writer) and GetKpProposal/GetKpProposalHTML/
// DownloadKpProposalPDF (readers): a field tag typo on any side would silently
// break tenant isolation or artifact availability instead of failing loudly.
func TestKpProposalCacheEntryRoundTrip(t *testing.T) {
	want := kpProposalCacheEntry{
		ProjectID:     "37fecd3e-dde6-4714-9691-08f1970d6d2f",
		EnvironmentID: "1971aabb-5682-4ae9-a49a-58cf72c10a76",
		Title:         "SaaS-платформа",
		PageCount:     9,
		QAStatus:      "PASS",
		HasHTML:       true,
		HasPDF:        true,
		Prototype: &kpAgentPrototype{
			URL:             "https://kp.example.com/p/public-id/",
			QAStatus:        "PASS",
			ScreenCount:     12,
			RendererVersion: "app-prototype-v2",
		},
	}

	body, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got kpProposalCacheEntry
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ProjectID != want.ProjectID || got.EnvironmentID != want.EnvironmentID || got.Title != want.Title ||
		got.PageCount != want.PageCount || got.QAStatus != want.QAStatus || got.HasHTML != want.HasHTML || got.HasPDF != want.HasPDF {
		t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if got.Prototype == nil || *got.Prototype != *want.Prototype {
		t.Fatalf("prototype round trip mismatch\ngot:  %#v\nwant: %#v", got.Prototype, want.Prototype)
	}
}

func TestKpTenantFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validProjectID := "37fecd3e-dde6-4714-9691-08f1970d6d2f"
	validEnvironmentID := "1971aabb-5682-4ae9-a49a-58cf72c10a76"

	cases := []struct {
		name          string
		projectID     any
		environmentID any
		setProjectID  bool
		setEnvID      bool
		wantOK        bool
	}{
		{name: "valid", projectID: validProjectID, environmentID: validEnvironmentID, setProjectID: true, setEnvID: true, wantOK: true},
		{name: "missing project id", setEnvID: true, environmentID: validEnvironmentID, wantOK: false},
		{name: "invalid project id", projectID: "not-a-uuid", setProjectID: true, setEnvID: true, environmentID: validEnvironmentID, wantOK: false},
		{name: "missing environment id", projectID: validProjectID, setProjectID: true, wantOK: false},
		{name: "invalid environment id", projectID: validProjectID, setProjectID: true, environmentID: "not-a-uuid", setEnvID: true, wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/kp-proposals/KP-1", nil)
			if tc.setProjectID {
				c.Set("project_id", tc.projectID)
			}
			if tc.setEnvID {
				c.Set("environment_id", tc.environmentID)
			}

			h := kpTestHandler(config.BaseConfig{})
			gotProjectID, gotEnvironmentID, ok := h.kpTenantFromContext(c)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (recorder: %s)", ok, tc.wantOK, recorder.Body.String())
			}
			if tc.wantOK && (gotProjectID != validProjectID || gotEnvironmentID != validEnvironmentID) {
				t.Fatalf("got (%q, %q), want (%q, %q)", gotProjectID, gotEnvironmentID, validProjectID, validEnvironmentID)
			}
		})
	}
}

// Without centralRedis configured, every KP artifact endpoint must degrade to a
// clean 404 rather than panicking on a nil client — this is the real shape of a
// misconfigured deploy (e.g. centralRedis wiring broke) and of any request for a
// requestId whose cache entry expired.
func TestKpArtifactEndpointsWithoutRedisReturnNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newContext := func(path string) (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		c.Params = gin.Params{{Key: "requestId", Value: "KP-20260805-ABC123"}}
		c.Set("project_id", "37fecd3e-dde6-4714-9691-08f1970d6d2f")
		c.Set("environment_id", "1971aabb-5682-4ae9-a49a-58cf72c10a76")
		return c, recorder
	}

	h := kpTestHandler(config.BaseConfig{KpAgentURL: "http://unused.invalid"})

	t.Run("GetKpProposal", func(t *testing.T) {
		c, recorder := newContext("/v1/kp-proposals/KP-20260805-ABC123")
		h.GetKpProposal(c)
		assertKpArtifactNotFound(t, recorder)
	})
	t.Run("GetKpProposalHTML", func(t *testing.T) {
		c, recorder := newContext("/v1/kp-proposals/KP-20260805-ABC123/html")
		h.GetKpProposalHTML(c)
		assertKpArtifactNotFound(t, recorder)
	})
	t.Run("DownloadKpProposalPDF", func(t *testing.T) {
		c, recorder := newContext("/v1/kp-proposals/KP-20260805-ABC123/pdf")
		h.DownloadKpProposalPDF(c)
		assertKpArtifactNotFound(t, recorder)
	})
}

func assertKpArtifactNotFound(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v: %s", err, recorder.Body.String())
	}
	if envelope.Data.Error.Code != "KP_ARTIFACT_NOT_FOUND" {
		t.Fatalf("error code = %q, want KP_ARTIFACT_NOT_FOUND: %s", envelope.Data.Error.Code, recorder.Body.String())
	}
}

func TestGetKpProposalInvalidRequestIDReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/kp-proposals/not-a-kp-id", nil)
	c.Params = gin.Params{{Key: "requestId", Value: "not-a-kp-id"}}
	c.Set("project_id", "37fecd3e-dde6-4714-9691-08f1970d6d2f")
	c.Set("environment_id", "1971aabb-5682-4ae9-a49a-58cf72c10a76")

	h := kpTestHandler(config.BaseConfig{})
	h.GetKpProposal(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}
