package handlers

import (
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) LoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceId := c.GetHeader("Resource-Id")
		environmentId := c.GetHeader("Environment-Id")
		projectId := c.DefaultQuery("project-id", "")

		c.Set("resource_id", resourceId)
		c.Set("environment_id", environmentId)
		c.Set("project_id", projectId)
		c.Next()
	}
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			resourceId    = c.GetHeader("Resource-Id")
			environmentId = c.GetHeader("Environment-Id")
			bearerToken   = c.GetHeader("Authorization")
			projectId     = c.Query("project-id")
		)

		if len(bearerToken) == 0 {
			c.Set("resource_id", resourceId)
			c.Set("environment_id", environmentId)
			c.Set("project_id", projectId)
			c.Next()
			return
		}

		strArr := strings.Split(bearerToken, " ")
		if len(strArr) < 1 && (strArr[0] != "Bearer" && strArr[0] != "API-KEY") {
			_ = c.AbortWithError(nethttp.StatusForbidden, errors.New("token error: wrong format"))
			return
		}

		switch strArr[0] {
		case "Bearer":
			res, ok := h.hasAccess(c)
			if !ok {
				c.Abort()
				return
			}

			resourceId = c.GetHeader("Resource-Id")
			environmentId = c.GetHeader("Environment-Id")
			projectId = c.Query("Project-Id")

			if res.ProjectId != "" {
				projectId = res.ProjectId
			}
			if res.EnvId != "" {
				environmentId = res.EnvId
			}

			c.Set("user_id", res.UserIdAuth)
			c.Set("session_id", res.Id)
		case "API-KEY":
			app_id := c.GetHeader("X-API-KEY")
			apikeys, err := h.services.ApiKeysService().GetEnvID(
				c.Request.Context(),
				&auth_service.GetReq{
					Id: app_id,
				},
			)
			if err != nil {
				h.handleResponse(c, http.BadRequest, err.Error())
				c.Abort()
				return
			}

			resource, err := h.services.ResourceService().GetResourceByEnvID(
				c.Request.Context(),
				&company_service.GetResourceByEnvIDRequest{
					EnvId: apikeys.GetEnvironmentId(),
				},
			)
			if err != nil {
				h.handleResponse(c, http.BadRequest, err.Error())
				c.Abort()
				return
			}

			resourceId = resource.GetResource().GetId()
			environmentId = apikeys.GetEnvironmentId()
			projectId = apikeys.GetProjectId()
		default:
			if !strings.Contains(c.Request.URL.Path, "api") {
				err := errors.New("error invalid authorization method")
				h.handleResponse(c, http.BadRequest, err.Error())
				c.Abort()
			} else {
				c.JSON(401, struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    401,
					Message: "The request requires an user authentication.",
				})
				c.Abort()
			}
		}

		c.Set("resource_id", resourceId)
		c.Set("environment_id", environmentId)
		c.Set("project_id", projectId)
		c.Next()
	}
}

func (h *Handler) hasAccess(c *gin.Context) (*auth_service.V2HasAccessUserRes, bool) {
	bearerToken := c.GetHeader("Authorization")

	strArr := strings.Split(bearerToken, " ")

	if len(strArr) != 2 || strArr[0] != "Bearer" {
		h.log.Error("---ERR->HasAccess->Unexpected token format")
		h.handleResponse(c, http.Forbidden, "token error: wrong format")
		return nil, false
	}
	accessToken := strArr[1]
	resp, err := h.services.SessionService().V2HasAccessUser(
		c.Request.Context(),
		&auth_service.V2HasAccessUserReq{
			AccessToken: accessToken,
			Path:        helper.GetURLWithTableSlug(c),
			Method:      c.Request.Method,
		},
	)
	if err != nil {
		if message, blocking := config.BlockingStatusMessage(err); blocking {
			h.handleResponse(c, http.BadRequest, message)
			return nil, false
		}
		permissionErrors := map[string]struct{}{
			status.Error(codes.PermissionDenied, config.PermissionDenied).Error(): {},
		}
		if _, exists := permissionErrors[err.Error()]; exists {
			h.handleResponse(c, http.BadRequest, err.Error())
			return nil, false
		}
		errr := status.Error(codes.InvalidArgument, "Session has been expired")
		if errr.Error() == err.Error() {
			h.handleError(c, http.Forbidden, err)
			return nil, false
		}
		h.handleError(c, http.Unauthorized, err)
		return nil, false
	}

	return resp, true
}
