package v2

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	apb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"net/http"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
)

func (h *HandlerV2) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			res         = &auth_service.V2HasAccessUserRes{}
			ok          bool
			bearerToken = c.GetHeader("Authorization")
			strArr      = strings.Split(bearerToken, " ")
		)

		if len(strArr) < 1 && (strArr[0] != "Bearer" && strArr[0] != "API-KEY") {
			h.log.Error("---ERR->Unexpected token format")
			_ = c.AbortWithError(http.StatusForbidden, config.ErrTokenFormat)
			return
		}
		switch strArr[0] {
		case "Bearer":
			res, ok = h.hasAccess(c)
			if !ok {
				c.Abort()
				return
			}
			resourceId := c.GetHeader("Resource-Id")
			environmentId := c.GetHeader("Environment-Id")
			projectId := c.Query("Project-Id")
			userId := c.Query("User-Id")

			if res.ProjectId != "" {
				projectId = res.ProjectId
			}
			if res.EnvId != "" {
				environmentId = res.EnvId
			}
			if res.UserId != "" {
				userId = res.UserIdAuth
			}

			c.Set("resource_id", resourceId)
			c.Set("environment_id", environmentId)
			c.Set("project_id", projectId)
			c.Set("user_id", userId)
			c.Set("role_id", res.RoleId)
			c.Set("token", strArr[1])
		case "API-KEY":
			app_id := c.GetHeader("X-API-KEY")

			if app_id == "" {
				h.HandleResponse(c, status_http.Unauthorized, "The request requires an user authentication.")
				c.Abort()
				return
			}

			var (
				appIdKey, resourceAppIdKey = app_id, app_id + "resource"

				err      error
				apiJson  []byte
				apikeys  = &apb.GetRes{}
				resource = &pb.GetResourceByEnvIDResponse{}
			)

			var appWaitkey = config.CACHE_WAIT + "-appID"
			_, appIdOk := h.cache.Get(appWaitkey)
			if !appIdOk {
				h.cache.Add(appWaitkey, []byte(appWaitkey), config.REDIS_KEY_TIMEOUT)
			}

			if appIdOk {
				ctx, cancel := context.WithTimeout(c.Request.Context(), config.REDIS_WAIT_TIMEOUT)
				defer cancel()

				for {
					appIdBody, ok := h.cache.Get(appIdKey)
					if ok {
						apiJson = appIdBody
						err = json.Unmarshal(appIdBody, &apikeys)
						if err != nil {
							h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
							c.Abort()
							return
						}
					}

					if apikeys.AppId != "" {
						break
					}

					if ctx.Err() == context.DeadlineExceeded {
						break
					}

					time.Sleep(config.REDIS_SLEEP)
				}
			}

			if apikeys.AppId == "" {
				apikeys, err = h.authService.ApiKey().GetEnvID(
					c.Request.Context(),
					&apb.GetReq{
						Id: app_id,
					},
				)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, err.Error())
					c.Abort()
					return
				}

				apiJson, err = json.Marshal(apikeys)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
					c.Abort()
					return
				}

				go func() {
					h.cache.Add(appIdKey, apiJson, config.REDIS_TIMEOUT)
				}()
			}

			var resourceWaitKey = config.CACHE_WAIT + "-resource"
			_, resourceOk := h.cache.Get(resourceWaitKey)
			if !resourceOk {
				h.cache.Add(resourceWaitKey, []byte(resourceWaitKey), config.REDIS_KEY_TIMEOUT)
			}

			if resourceOk {
				ctx, cancel := context.WithTimeout(c.Request.Context(), config.REDIS_WAIT_TIMEOUT)
				defer cancel()

				for {
					resourceBody, ok := h.cache.Get(resourceAppIdKey)
					if ok {
						err = json.Unmarshal(resourceBody, &resource)
						if err != nil {
							h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
							c.Abort()
							return
						}
					}

					if resource.Resource != nil {
						break
					}

					if ctx.Err() == context.DeadlineExceeded {
						break
					}

					time.Sleep(config.REDIS_SLEEP)
				}
			}

			if resource.Resource == nil {
				resource, err = h.companyServices.Resource().GetResourceByEnvID(
					c.Request.Context(),
					&pb.GetResourceByEnvIDRequest{
						EnvId: apikeys.GetEnvironmentId(),
					},
				)
				if err != nil {
					h.HandleResponse(c, status_http.BadRequest, err.Error())
					c.Abort()
					return
				}

				go func() {
					resourceBody, err := json.Marshal(resource)
					if err != nil {
						h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
						return
					}
					h.cache.Add(resourceAppIdKey, resourceBody, config.REDIS_TIMEOUT)
				}()
			}

			data := make(map[string]any)
			err = json.Unmarshal(apiJson, &data)
			if err != nil {
				h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
				c.Abort()
				return
			}

			resourceBody, err := json.Marshal(resource)
			if err != nil {
				h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
				return
			}

			if message, blocking := config.ProjectStatusMessage(resource.ProjectStatus); blocking {
				h.HandleResponse(c, status_http.BadRequest, message)
				c.Abort()
				return
			}

			c.Set("auth", models.AuthData{Type: "API-KEY", Data: data})
			c.Set("resource_id", resource.GetResource().GetId())
			c.Set("environment_id", apikeys.GetEnvironmentId())
			c.Set("project_id", apikeys.GetProjectId())
			c.Set("resource", string(resourceBody))
		default:
			if !strings.Contains(c.Request.URL.Path, "api") {
				err := errors.New("error invalid authorization method")
				h.log.Error("--AuthMiddleware--", logger.Error(err))
				h.HandleResponse(c, status_http.BadRequest, err.Error())
				c.Abort()
			} else {

				err := errors.New("error invalid authorization method")
				h.log.Error("--AuthMiddleware--", logger.Error(err))
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
		c.Set("Auth", res)
		c.Set("namespace", h.baseConf.UcodeNamespace)
		c.Next()

	}
}
