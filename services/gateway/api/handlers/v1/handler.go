package v1

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/gemini"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/caching"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/vault"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"github.com/Ucode-io/ucode-opensource/services/gateway/storage"

	"github.com/gin-gonic/gin"
	go_redis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HandlerV1 struct {
	baseConf          config.BaseConfig
	projectConfs      map[string]config.Config
	log               logger.LoggerI
	services          services.ServiceNodesI
	companyServices   services.CompanyServiceI
	authService       services.AuthServiceManagerI
	redis             storage.RedisStorageI
	centralRedis      *go_redis.Client
	cache             *caching.ExpiringLRUCache
	rateLimiter       *util.ApiKeyRateLimiter
	vault             vault.VaultClient
	geminiKeyPool     *gemini.KeyPool
	aiEditPromptStore aiEditPromptStore
}

func NewHandlerV1(baseConf config.BaseConfig, projectConfs map[string]config.Config, log logger.LoggerI, svcs services.ServiceNodesI, cmpServ services.CompanyServiceI, authService services.AuthServiceManagerI, redis storage.RedisStorageI, centralRedis *go_redis.Client, cache *caching.ExpiringLRUCache, limiter *util.ApiKeyRateLimiter, vaultClient vault.VaultClient) HandlerV1 {
	h := HandlerV1{
		baseConf:          baseConf,
		projectConfs:      projectConfs,
		log:               log,
		services:          svcs,
		companyServices:   cmpServ,
		authService:       authService,
		redis:             redis,
		centralRedis:      centralRedis,
		cache:             cache,
		rateLimiter:       limiter,
		vault:             vaultClient,
		aiEditPromptStore: grpcAIEditPromptStore{},
	}
	if len(baseConf.GeminiAPIKeys) > 0 {
		pool, err := gemini.NewKeyPool(baseConf.GeminiAPIKeys)
		if err != nil {
			log.Error(fmt.Sprintf("[gemini] key pool error: %v", err))
		} else {
			h.geminiKeyPool = pool
			log.Info(fmt.Sprintf("[gemini] key pool ready: %d keys", len(baseConf.GeminiAPIKeys)))
		}
	}
	return h
}

func (h *HandlerV1) GetProjectSrvc(c context.Context, projectId string, nodeType string) (services.ServiceManagerI, error) {
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

func (h *HandlerV1) BaseConf() config.BaseConfig               { return h.baseConf }
func (h *HandlerV1) AuthService() services.AuthServiceManagerI { return h.authService }

func (h *HandlerV1) HandleResponse(c *gin.Context, status status_http.Status, data any) {
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

func (h *HandlerV1) handleError(c *gin.Context, statusHttp status_http.Status, err error) {
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
	} else if st.Code() == codes.FailedPrecondition {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:        statusHttp.Status,
			Description:   st.String(),
			Data:          "Cannot drop or modify the object because dependent objects exist.",
			CustomMessage: statusHttp.CustomMessage,
		})
	} else if statusHttp.Status == status_http.Unauthorized.Status {
		c.JSON(http.StatusUnauthorized, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        capitalFirstLetter(st.Message()),
		})
	} else if statusHttp.Status == status_http.Forbidden.Status {
		c.JSON(http.StatusForbidden, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        capitalFirstLetter(st.Message()),
		})
	} else if st.Err() != nil {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:        statusHttp.Status,
			Description:   st.String(),
			Data:          capitalFirstLetter(st.Message()),
			CustomMessage: statusHttp.CustomMessage,
		})
	}
}

func capitalFirstLetter(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

func (h *HandlerV1) getOffsetParam(c *gin.Context) (offset int, err error) {
	offsetStr := c.DefaultQuery("offset", h.baseConf.DefaultOffset)
	return strconv.Atoi(offsetStr)
}

func (h *HandlerV1) getLimitParam(c *gin.Context) (limit int, err error) {
	limitStr := c.DefaultQuery("limit", h.baseConf.DefaultLimit)
	return strconv.Atoi(limitStr)
}

func (h *HandlerV1) getLimitParamWithoutDefault(c *gin.Context) (limit int, err error) {
	limitStr := c.DefaultQuery("limit", "0")
	return strconv.Atoi(limitStr)
}

func (h *HandlerV1) versionHistory(req *models.CreateVersionHistoryRequest) error {
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
			&auth.UserPrimaryKey{
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
			MethodApi:         req.MethodApi,
			TimeStarted:       req.TimeStarted,
			TimeCompleted:     req.TimeCompleted,
			Duration:          req.Duration,
			StatusCode:        int64(req.StatusCode),
		},
	)
	if err != nil {
		h.log.Error("Error while create version history", logger.Any("err", err))
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

func (h *HandlerV1) versionHistoryGo(c *gin.Context, req *models.CreateVersionHistoryRequest) error {
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
			&auth.UserPrimaryKey{
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

	if req.StatusCode == 0 && c != nil {
		req.StatusCode = c.Writer.Status()
	}

	if req.TimeCompleted == "" {
		req.TimeCompleted = time.Now().Format(time.RFC3339)
	}
	if req.Duration == 0 && req.TimeStarted != "" {
		if t, err := time.Parse(time.RFC3339, req.TimeStarted); err == nil {
			req.Duration = time.Since(t).Milliseconds()
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
			MethodApi:         req.MethodApi,
			TimeStarted:       req.TimeStarted,
			TimeCompleted:     req.TimeCompleted,
			Duration:          req.Duration,
			StatusCode:        int64(req.StatusCode),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (h *HandlerV1) MakeProxy(c *gin.Context, proxyUrl, path string) (err error) {
	var req = c.Request

	proxy, err := url.Parse(proxyUrl)
	if err != nil {
		h.log.Error("error in parse addr: %v", logger.Error(err))
		c.String(http.StatusInternalServerError, "error")
		return
	}

	req.URL.Scheme = proxy.Scheme
	req.URL.Host = proxy.Host
	req.URL.Path = path
	transport := http.DefaultTransport

	resp, err := transport.RoundTrip(req)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)
	_, _ = bufio.NewReader(resp.Body).WriteTo(c.Writer)
	return
}
