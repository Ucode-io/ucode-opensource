package metaads

import (
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
	go_redis "github.com/go-redis/redis/v8"
)

type Handler struct {
	service      *service
	maxRangeDays int
	log          logger.LoggerI
}

func NewHandler(baseConf config.BaseConfig, redisClient *go_redis.Client, log logger.LoggerI) Handler {
	if baseConf.MetaAdsAdAccountID == "" || baseConf.MetaAdsAccessToken == "" {
		return Handler{maxRangeDays: baseConf.MetaAdsMaxRangeDays, log: log}
	}
	timeout := time.Duration(baseConf.MetaAdsRequestTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cacheTTL := time.Duration(baseConf.MetaAdsCacheTTLSeconds) * time.Second
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Minute
	}
	client := newGraphClient(
		baseConf.MetaAdsGraphBaseURL,
		baseConf.MetaAdsGraphVersion,
		baseConf.MetaAdsAdAccountID,
		baseConf.MetaAdsAccessToken,
		timeout,
	)
	cache := redisDashboardCache{client: redisClient}
	return Handler{
		service:      newService(client, cache, cacheTTL, baseConf.MetaAdsAdAccountID, baseConf.MetaAdsLeadActionTypes, baseConf.MetaAdsAttributionWindows, log),
		maxRangeDays: baseConf.MetaAdsMaxRangeDays,
		log:          log,
	}
}

func (h Handler) Dashboard(c *gin.Context) {
	if h.service == nil {
		h.respond(c, status_http.ServiceUnavailable, "Meta Ads integration is not configured")
		return
	}
	query, err := parseDashboardQuery(c, h.maxRangeDays)
	if err != nil {
		h.respond(c, status_http.BadRequest, err.Error())
		return
	}
	response, err := h.service.dashboard(c.Request.Context(), query, c.Query("prefer_cache") == "true")
	if err != nil {
		h.log.Error("meta ads: dashboard request failed", logger.Error(err))
		h.respond(c, status_http.BadGateway, "Meta Ads data is temporarily unavailable")
		return
	}
	h.respond(c, status_http.OK, response)
}

func (h Handler) Account(c *gin.Context) {
	if h.service == nil {
		h.respond(c, status_http.ServiceUnavailable, "Meta Ads integration is not configured")
		return
	}
	account, err := h.service.account(c.Request.Context())
	if err != nil {
		h.log.Error("meta ads: account smoke test failed", logger.Error(err))
		h.respond(c, status_http.BadGateway, "Meta Ads account is temporarily unavailable")
		return
	}
	h.respond(c, status_http.OK, account)
}

func (h Handler) respond(c *gin.Context, status status_http.Status, data any) {
	c.JSON(status.Code, status_http.Response{
		Status:        status.Status,
		Description:   status.Description,
		Data:          data,
		CustomMessage: status.CustomMessage,
	})
}
