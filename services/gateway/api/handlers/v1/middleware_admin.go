package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *HandlerV1) AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if cast.ToBool(c.GetHeader("redirect")) {
			var authData = models.AuthData{}
			err := json.Unmarshal([]byte(c.GetHeader("auth")), &authData)
			if err != nil {
				h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
				c.Abort()
				return
			}

			c.Set("auth", authData)
			c.Set("resource_id", c.GetHeader("resource_id"))
			c.Set("environment_id", c.GetHeader("environment_id"))
			c.Set("project_id", c.GetHeader("project_id"))
		} else {
			var (
				ok          bool
				res         = &auth.HasAccessSuperAdminRes{}
				data        = make(map[string]any)
				bearerToken = c.GetHeader("Authorization")
				strArr      = strings.Split(bearerToken, " ")
			)

			if len(strArr) < 1 || (strArr[0] != "Bearer" && strArr[0] != "API-KEY") {
				h.log.Error("---ERR->Unexpected token format")
				_ = c.AbortWithError(http.StatusForbidden, config.ErrTokenFormat)
				return
			}

			switch strArr[0] {
			case "Bearer":
				res, ok = h.adminHasAccess(c)
				if !ok {
					_ = c.AbortWithError(401, errors.New("unauthorized"))
					return
				}

				resourceId := c.GetHeader("Resource-Id")
				environmentId := c.GetHeader("Environment-Id")
				projectId := c.DefaultQuery("project-id", "")

				if res.ProjectId != "" {
					projectId = res.ProjectId
				}
				if res.EnvId != "" {
					environmentId = res.EnvId
				}

				apiJson, err := json.Marshal(res)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
					c.Abort()
					return
				}

				err = json.Unmarshal(apiJson, &data)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
					c.Abort()
					return
				}

				c.Set("auth", models.AuthData{
					Type: "BEARER",
					Data: data,
				})

				c.Set("environment_id", environmentId)
				c.Set("resource_id", resourceId)
				c.Set("project_id", projectId)
				c.Set("Auth_Admin", res)
				c.Set("user_id", res.UserId)
			case "API-KEY":
				appId := c.GetHeader("X-API-KEY")
				apiKey, err := h.authService.ApiKey().GetEnvID(
					c.Request.Context(),
					&auth.GetReq{
						Id: appId,
					},
				)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, err.Error())
					c.Abort()
					return
				}
				apiJson, err := json.Marshal(apiKey)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
					c.Abort()
					return
				}
				err = json.Unmarshal(apiJson, &data)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
					c.Abort()
					return
				}
				c.Set("auth", models.AuthData{
					Type: "API-KEY",
					Data: data,
				})
				c.Set("environment_id", apiKey.GetEnvironmentId())
				c.Set("project_id", apiKey.GetProjectId())
			default:
				err := errors.New("error invalid authorization method")
				h.log.Error("--AuthMiddleware--", logger.Error(err))
				h.HandleResponse(c, status_http.BadRequest, err.Error())
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func (h *HandlerV1) BearerOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			h.HandleResponse(c, status_http.Unauthorized, "Bearer authentication is required")
			c.Abort()
			return
		}

		// AdminAuthMiddleware is responsible for validating the token and sets
		// Auth_Admin only on its authenticated Bearer path. In particular, its
		// trusted internal redirect mode deliberately skips token validation, so
		// checking the header shape alone would let that mode reach these
		// user-facing prompt settings endpoints.
		authAdmin, ok := c.Get("Auth_Admin")
		if !ok {
			h.HandleResponse(c, status_http.Unauthorized, "validated Bearer authentication is required")
			c.Abort()
			return
		}
		if _, ok = authAdmin.(*auth.HasAccessSuperAdminRes); !ok {
			h.HandleResponse(c, status_http.Unauthorized, "validated Bearer authentication is required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *HandlerV1) adminHasAccess(c *gin.Context) (*auth.HasAccessSuperAdminRes, bool) {
	bearerToken := c.GetHeader("Authorization")
	strArr := strings.Split(bearerToken, " ")
	if len(strArr) != 2 || strArr[0] != "Bearer" {
		h.HandleResponse(c, status_http.Forbidden, "token error: wrong format")
		return nil, false
	}
	accessToken := strArr[1]
	service, conn, err := h.authService.Session(c)
	if err != nil {
		return nil, false
	}
	defer conn.Close()

	path, tableSlug := helper.GetURLWithTableSlug(c)

	resp, err := service.HasAccessSuperAdmin(
		c.Request.Context(),
		&auth.HasAccessSuperAdminReq{
			AccessToken: accessToken,
			Path:        path,
			Method:      c.Request.Method,
			TableSlug:   tableSlug,
		},
	)
	if err != nil {
		if message, blocking := config.BlockingStatusMessage(err); blocking {
			h.HandleResponse(c, status_http.BadRequest, message)
			return nil, false
		}
		permissionErrors := map[string]struct{}{
			status.Error(codes.PermissionDenied, config.PermissionDenied).Error(): {},
		}
		if _, exists := permissionErrors[err.Error()]; exists {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return nil, false
		}
		errr := status.Error(codes.InvalidArgument, "User has been expired")
		if errr.Error() == err.Error() {
			h.HandleResponse(c, status_http.Forbidden, err.Error())
			return nil, false
		}
		h.HandleResponse(c, status_http.Unauthorized, err.Error())
		return nil, false
	}

	return resp, true
}

func (h *HandlerV1) adminAuthInfo(c *gin.Context) (result *auth.HasAccessSuperAdminRes, err error) {
	data, ok := c.Get("Auth_Admin")

	if !ok {
		h.HandleResponse(c, status_http.Forbidden, "token error: wrong format")
		c.Abort()
		return nil, config.ErrTokenFormat
	}
	accessResponse, ok := data.(*auth.HasAccessSuperAdminRes)
	if !ok {
		h.HandleResponse(c, status_http.Forbidden, "token error: wrong format")
		c.Abort()
		return nil, config.ErrTokenFormat
	}

	return accessResponse, nil
}
