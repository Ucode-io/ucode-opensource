package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"

	"github.com/gin-gonic/gin"
)

// ValidateApiKey godoc
// @ID validate_api_key
// @Router /v1/api-key/validate [GET]
// @Summary Validate X-API-KEY
// @Description Lightweight validation of an X-API-KEY. Returns the project/environment bound to the key. Public (no AuthMiddleware) so a client can verify a key before opening a session.
// @Tags ApiKey
// @Accept json
// @Produce json
// @Param X-API-KEY header string true "API key"
// @Success 200 {object} status_http.Response{data=models.ValidateApiKeyResponse} "valid"
// @Failure 401 {object} status_http.Response{data=string} "invalid or missing key"
func (h *HandlerV1) ValidateApiKey(c *gin.Context) {
	appID := c.GetHeader("X-API-KEY")
	if appID == "" {
		h.HandleResponse(c, status_http.Unauthorized, "X-API-KEY header is required")
		return
	}

	apiKey, err := h.authService.ApiKey().GetEnvID(
		c.Request.Context(), &auth.GetReq{Id: appID},
	)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "invalid api key")
		return
	}

	if apiKey.GetDisable() {
		h.HandleResponse(c, status_http.Unauthorized, "api key is disabled")
		return
	}

	h.HandleResponse(c, status_http.OK, models.ValidateApiKeyResponse{
		Valid:         true,
		AppId:         apiKey.GetAppId(),
		ProjectId:     apiKey.GetProjectId(),
		EnvironmentId: apiKey.GetEnvironmentId(),
	})
}