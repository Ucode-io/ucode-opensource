package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
)

func TestBearerOnlyMiddlewareRejectsRedirectContextWithFakeBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			h := &HandlerV1{log: logger.NewLogger("bearer-only-test", logger.LevelError)}

			router := gin.New()
			router.Use(func(c *gin.Context) {
				// This is the context shape populated by AdminAuthMiddleware's
				// internal redirect branch. It is not proof that a Bearer token
				// was validated.
				c.Set("auth", models.AuthData{Type: "BEARER"})
				c.Set("project_id", "spoofed-project")
				c.Next()
			})
			router.Use(h.BearerOnlyMiddleware())
			called := false
			handler := func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			}
			router.GET("/", handler)
			router.DELETE("/", handler)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, "/", nil)
			request.Header.Set("Authorization", "Bearer fake-token")
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			if called {
				t.Fatal("protected handler must not run without validated Auth_Admin context")
			}
		})
	}
}

func TestBearerOnlyMiddlewareAcceptsValidatedBearerContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HandlerV1{log: logger.NewLogger("bearer-only-test", logger.LevelError)}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("Auth_Admin", &auth.HasAccessSuperAdminRes{UserId: "user-id"})
		c.Next()
	})
	router.Use(h.BearerOnlyMiddleware())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer validated-token")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
