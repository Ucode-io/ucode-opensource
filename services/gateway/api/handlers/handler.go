package handlers

import (
	v1 "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/v1"
	v2 "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/v2"
	v3 "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/v3"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/caching"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/vault"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"github.com/Ucode-io/ucode-opensource/services/gateway/storage"

	"github.com/gin-gonic/gin"
	go_redis "github.com/go-redis/redis/v8"
)

type Handler struct {
	baseConf        config.BaseConfig
	projectConfs    map[string]config.Config
	log             logger.LoggerI
	services        services.ServiceNodesI
	companyServices services.CompanyServiceI
	authService     services.AuthServiceManagerI
	redis           storage.RedisStorageI
	centralRedis    *go_redis.Client
	V1              v1.HandlerV1
	V2              v2.HandlerV2
	V3              v3.HandlerV3
	cache           *caching.ExpiringLRUCache
}

func NewHandler(baseConf config.BaseConfig, projectConfs map[string]config.Config, log logger.LoggerI, svcs services.ServiceNodesI, cmpServ services.CompanyServiceI, authService services.AuthServiceManagerI, redis storage.RedisStorageI, centralRedis *go_redis.Client, cache *caching.ExpiringLRUCache, limiter *util.ApiKeyRateLimiter, vaultClient vault.VaultClient) Handler {
	return Handler{
		baseConf:        baseConf,
		projectConfs:    projectConfs,
		log:             log,
		services:        svcs,
		companyServices: cmpServ,
		authService:     authService,
		redis:           redis,
		centralRedis:    centralRedis,
		V1:              v1.NewHandlerV1(baseConf, projectConfs, log, svcs, cmpServ, authService, redis, centralRedis, cache, limiter, vaultClient),
		V2:              v2.NewHandlerV2(baseConf, projectConfs, log, svcs, cmpServ, authService, redis, centralRedis, cache, limiter),
		V3: v3.NewHandlerV3(&v3.HandlerV3Config{
			BaseConf:        baseConf,
			ProjectConfs:    projectConfs,
			Log:             log,
			Services:        svcs,
			CompanyServices: cmpServ,
			AuthService:     authService,
			Redis:           redis,
			CentralRedis:    centralRedis,
			RateLimiter:     limiter,
			Cache:           cache,
		}),
		cache: cache,
	}
}

func (h *Handler) GetCompanyService(c *gin.Context) services.CompanyServiceI {
	return h.companyServices
}
