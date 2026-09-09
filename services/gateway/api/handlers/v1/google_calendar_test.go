package v1

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildGoogleCalendarOriginRedirectSuccess(t *testing.T) {
	state := googleCalendarOAuthState{
		ProjectID:     "37fecd3e-dde6-4714-9691-08f1970d6d2f",
		EnvironmentID: "1971aabb-5682-4ae9-a49a-58cf72c10a76",
		McpProjectID:  "58be0b19-371f-4ff6-af41-50878b12743c",
	}

	got := buildGoogleCalendarOriginRedirect("http://localhost:3000", true, "", state)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("unparseable redirect %q: %v", got, err)
	}
	if u.Scheme != "http" || u.Host != "localhost:3000" {
		t.Fatalf("expected popup to return to the opener origin, got %q", got)
	}
	if u.Path != googleCalendarSuccessPath {
		t.Fatalf("expected close-page path %q, got %q", googleCalendarSuccessPath, u.Path)
	}
	q := u.Query()
	if q.Get("google_calendar") != "success" {
		t.Fatalf("expected google_calendar=success, got %q", q.Get("google_calendar"))
	}
	if q.Get("mcp_project_id") != state.McpProjectID {
		t.Fatalf("expected mcp_project_id %q, got %q", state.McpProjectID, q.Get("mcp_project_id"))
	}
}

func TestBuildGoogleCalendarOriginRedirectErrorCarriesReason(t *testing.T) {
	got := buildGoogleCalendarOriginRedirect("https://app.ucode.run", false, "token_exchange_failed", googleCalendarOAuthState{})

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("unparseable redirect %q: %v", got, err)
	}
	if u.Path != googleCalendarErrorPath {
		t.Fatalf("expected error path %q, got %q", googleCalendarErrorPath, u.Path)
	}
	q := u.Query()
	if q.Get("google_calendar") != "error" || q.Get("reason") != "token_exchange_failed" {
		t.Fatalf("expected error+reason in query, got %q", u.RawQuery)
	}
}

func TestResolveGoogleCalendarFrontendOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		origin string
		refer  string
		want   string
	}{
		{name: "origin header", origin: "https://app.ucode.run", want: "https://app.ucode.run"},
		{name: "referer fallback", refer: "http://localhost:3000/projects/abc?tab=resources", want: "http://localhost:3000"},
		{name: "none", want: ""},
		{name: "non-http scheme ignored", origin: "ftp://evil", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/google-calendar/connect", nil)
			if tc.origin != "" {
				c.Request.Header.Set("Origin", tc.origin)
			}
			if tc.refer != "" {
				c.Request.Header.Set("Referer", tc.refer)
			}
			if got := resolveGoogleCalendarFrontendOrigin(c); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
