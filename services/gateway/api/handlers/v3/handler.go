package v3

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/caching"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"github.com/Ucode-io/ucode-opensource/services/gateway/storage"

	"github.com/gin-gonic/gin"
	go_redis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdditionalParams struct {
}

type HandlerV3 struct {
	baseConf        config.BaseConfig
	projectConfs    map[string]config.Config
	log             logger.LoggerI
	services        services.ServiceNodesI
	companyServices services.CompanyServiceI
	authService     services.AuthServiceManagerI
	redis           storage.RedisStorageI
	centralRedis    *go_redis.Client
	cache           *caching.ExpiringLRUCache
	rateLimiter     *util.ApiKeyRateLimiter
}

type HandlerV3Config struct {
	BaseConf        config.BaseConfig
	ProjectConfs    map[string]config.Config
	Log             logger.LoggerI
	Services        services.ServiceNodesI
	CompanyServices services.CompanyServiceI
	AuthService     services.AuthServiceManagerI
	Redis           storage.RedisStorageI
	CentralRedis    *go_redis.Client
	Cache           *caching.ExpiringLRUCache
	RateLimiter     *util.ApiKeyRateLimiter
}

func NewHandlerV3(cf *HandlerV3Config) HandlerV3 {
	return HandlerV3{
		baseConf:        cf.BaseConf,
		projectConfs:    cf.ProjectConfs,
		log:             cf.Log,
		services:        cf.Services,
		companyServices: cf.CompanyServices,
		authService:     cf.AuthService,
		redis:           cf.Redis,
		centralRedis:    cf.CentralRedis,
		cache:           cf.Cache,
		rateLimiter:     cf.RateLimiter,
	}
}

func (h *HandlerV3) BaseConf() config.BaseConfig               { return h.baseConf }
func (h *HandlerV3) AuthService() services.AuthServiceManagerI { return h.authService }

func (h *HandlerV3) GetProjectSrvc(c context.Context, projectId string, nodeType string) (services.ServiceManagerI, error) {
	if nodeType == config.ENTER_PRICE_TYPE {
		srvc, err := h.services.Get(projectId)
		if err != nil {
			return nil, err
		}

		return srvc, nil
	} else {
		srvc, err := h.services.Get(h.baseConf.UcodeNamespace)
		if err != nil {
			return nil, err
		}

		return srvc, nil
	}
}

func (h *HandlerV3) HandleResponse(c *gin.Context, status status_http.Status, data any) {
	switch code := status.Code; {
	case code < 400:
	default:
		h.log.Error(
			"response",
			logger.Int("code", status.Code),
			logger.String("status", status.Status),
			logger.Any("description", status.Description),
			logger.Any("data", data),
			logger.Any("custom_message", status.CustomMessage),
		)
	}

	c.JSON(status.Code, status_http.Response{
		Status:        status.Status,
		Description:   status.Description,
		Data:          data,
		CustomMessage: status.CustomMessage,
	})
}

func (h *HandlerV3) handleError(c *gin.Context, statusHttp status_http.Status, err error) {
	st, _ := status.FromError(err)
	if statusHttp.Status == status_http.BadRequest.Status {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:        statusHttp.Status,
			Description:   st.String(),
			Data:          "Invalid JSON",
			CustomMessage: statusHttp.CustomMessage,
		})
	} else if st.Code() == codes.AlreadyExists {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:        statusHttp.Status,
			Description:   st.String(),
			Data:          "This slug already exists. Please choose a unique one.",
			CustomMessage: statusHttp.CustomMessage,
		})
	} else if st.Code() == codes.InvalidArgument {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:        statusHttp.Status,
			Description:   st.String(),
			Data:          "Required data is not provided.",
			CustomMessage: statusHttp.CustomMessage,
		})
	} else if statusHttp.Status == status_http.Unauthorized.Status {
		c.JSON(http.StatusUnauthorized, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.SessionExpired,
		})
	} else if statusHttp.Status == status_http.Forbidden.Status {
		c.JSON(http.StatusForbidden, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.SessionExpired,
		})
	} else if st.Err() != nil {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:        statusHttp.Status,
			Description:   st.String(),
			Data:          st.Message(),
			CustomMessage: statusHttp.CustomMessage,
		})
	}
}

func (h *HandlerV3) getOffsetParam(c *gin.Context) (offset int, err error) {
	offsetStr := c.DefaultQuery("offset", h.baseConf.DefaultOffset)
	return strconv.Atoi(offsetStr)
}

func (h *HandlerV3) versionHistory(req *models.CreateVersionHistoryRequest) error {
	var (
		current  = map[string]any{"data": req.Current}
		previous = map[string]any{"data": req.Previous}
		request  = map[string]any{"data": req.Request}
		response = map[string]any{"data": req.Response}
		user     = ""
	)

	if req.Current == nil {
		current["data"] = make(map[string]any)
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]any)
	}
	if req.Request == nil {
		request["data"] = make(map[string]any)
	}
	if req.Response == nil {
		response["data"] = make(map[string]any)
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.authService.User().GetUserByID(
			context.Background(),
			&auth_service.UserPrimaryKey{
				Id: req.UserInfo,
			},
		)
		if err == nil {
			if info.Login != "" {
				user = info.Login
			} else {
				user = info.Phone
			}
		}
	}

	_, err := req.Services.GetBuilderServiceByType(req.NodeType).VersionHistory().Create(
		context.Background(),
		&obs.CreateVersionHistoryRequest{
			Id:           uuid.NewString(),
			ProjectId:    req.ProjectId,
			ActionSource: req.ActionSource,
			ActionType:   req.ActionType,
			Previus:      fromMapToString(previous),
			Current:      fromMapToString(current),
			Date:         time.Now().Format("2006-01-02T15:04:05.000Z"),
			UserInfo:     user,
			Request:      fromMapToString(request),
			Response:     fromMapToString(response),
			ApiKey:       req.ApiKey,
			Type:         req.Type,
			TableSlug:    req.TableSlug,
			VersionId:    req.VersionId,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func fromMapToString(req map[string]any) string {
	reqString, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	return string(reqString)
}

func (h *HandlerV3) versionHistoryGo(c *gin.Context, req *models.CreateVersionHistoryRequest) error {
	var (
		current  = map[string]any{"data": req.Current}
		previous = map[string]any{"data": req.Previous}
		request  = map[string]any{"data": req.Request}
		response = map[string]any{"data": req.Response}
		user     = ""
	)

	if req.Current == nil {
		current["data"] = make(map[string]any)
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]any)
	}
	if req.Request == nil {
		request["data"] = make(map[string]any)
	}
	if req.Response == nil {
		response["data"] = make(map[string]any)
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.authService.User().GetUserByID(
			context.Background(),
			&auth_service.UserPrimaryKey{
				Id: req.UserInfo,
			},
		)
		if err == nil {
			if info.Login != "" {
				user = info.Login
			} else {
				user = info.Phone
			}
		}
	}

	_, err := req.Services.GoObjectBuilderService().VersionHistory().Create(
		c,
		&nb.CreateVersionHistoryRequest{
			Id:                uuid.NewString(),
			ProjectId:         req.ProjectId,
			ActionSource:      req.ActionSource,
			ActionType:        req.ActionType,
			Previus:           fromMapToString(previous),
			Current:           fromMapToString(current),
			UsedEnvrironments: req.UsedEnvironments,
			Date:              time.Now().Format("2006-01-02T15:04:05.000Z"),
			UserInfo:          user,
			Request:           fromMapToString(request),
			Response:          fromMapToString(response),
			ApiKey:            req.ApiKey,
			Type:              req.Type,
			TableSlug:         req.TableSlug,
			VersionId:         req.VersionId,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (h *HandlerV3) GetService(namespace string) (services.ServiceManagerI, error) {
	return h.services.Get(namespace)
}
