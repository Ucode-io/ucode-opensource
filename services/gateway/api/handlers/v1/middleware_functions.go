package v1

import (
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *HandlerV1) hasAccess(c *gin.Context) (*auth.V2HasAccessUserRes, bool) {
	var (
		bearerToken = c.GetHeader("Authorization")
		strArr      = strings.Split(bearerToken, " ")
	)

	if len(strArr) != 2 || strArr[0] != "Bearer" {
		h.log.Error("---ERR->HasAccess->Unexpected token format")
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

	resp, err := service.V2HasAccessUser(
		c.Request.Context(), &auth.V2HasAccessUserReq{
			AccessToken:   accessToken,
			Path:          path,
			Method:        c.Request.Method,
			ProjectId:     c.Query("project-id"),
			EnvironmentId: c.GetHeader("Environment-Id"),
			TableSlug:     tableSlug,
		},
	)
	if err != nil {
		if message, blocking := config.BlockingStatusMessage(err); blocking {
			h.log.Error("---ERR->HasAccess->ProjectStatus--->", logger.Error(err))
			h.HandleResponse(c, status_http.BadRequest, message)
			return nil, false
		}
		permissionErrors := map[string]struct{}{
			status.Error(codes.PermissionDenied, config.PermissionDenied).Error(): {},
		}
		if _, exists := permissionErrors[err.Error()]; exists {
			h.log.Error("---ERR->HasAccess->Permission--->", logger.Error(err))
			h.HandleResponse(c, status_http.BadRequest, "permission denied")
			return nil, false
		}
		errr := status.Error(codes.InvalidArgument, "User has been expired")
		if errr.Error() == err.Error() {
			h.log.Error("---ERR->HasAccess->User Expired-->")
			h.HandleResponse(c, status_http.Forbidden, err.Error())
			return nil, false
		}
		errr = status.Error(codes.Unavailable, "User not access environment")
		if errr.Error() == err.Error() {
			h.log.Error("---ERR->HasAccess->User not access environment-->")
			h.HandleResponse(c, status_http.Unauthorized, err.Error())
			return nil, false
		}

		h.log.Error("---ERR->HasAccess->Session->V2HasAccessUser--->", logger.Error(err))
		h.handleError(c, status_http.Unauthorized, err)
		return nil, false
	}

	return resp, true
}

func (h *HandlerV1) GetAuthInfo(c *gin.Context) (result *auth.V2HasAccessUserRes, err error) {
	data, ok := c.Get("Auth")
	if !ok {
		h.HandleResponse(c, status_http.Forbidden, "token error: wrong format")
		c.Abort()
		return nil, config.ErrTokenFormat
	}

	accessResponse, ok := data.(*auth.V2HasAccessUserRes)
	if !ok {
		h.HandleResponse(c, status_http.Forbidden, "token error: wrong format")
		c.Abort()
		return nil, config.ErrTokenFormat
	}

	return accessResponse, nil
}

func (h *HandlerV1) GetAuthAdminInfo(c *gin.Context) (result *auth.HasAccessSuperAdminRes, err error) {
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
