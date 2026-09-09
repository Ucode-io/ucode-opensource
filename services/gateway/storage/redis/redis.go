package redis

import (
	"context"
	"fmt"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/storage"

	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/go-redis/redis/v8"
)

type Storage struct {
	pool map[string]*redis.Client
}

func NewRedis(cfg map[string]config.Config, log logger.LoggerI) storage.RedisStorageI {
	redisPool := make(map[string]*redis.Client)

	for k, v := range cfg {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", v.GetRequestRedisHost, v.GetRequestRedisPort),
			Password: v.GetRequestRedisPassword,
			DB:       v.GetRequestRedisDatabase,
		})

		err := client.Ping(context.Background()).Err()
		if err != nil {
			log.Error("error connecting to redis", logger.String("project", k), logger.Error(err), logger.String("host", v.GetRequestRedisHost), logger.String("port", v.GetRequestRedisPort))
		} else {
			log.Info("successfully connected to redis", logger.String("project", k), logger.String("host", v.GetRequestRedisHost), logger.String("port", v.GetRequestRedisPort))
		}

		redisPool[k] = client
	}

	return Storage{
		pool: redisPool,
	}
}

func (s Storage) SetX(ctx context.Context, key string, value string, duration time.Duration, projectId string, nodeType string) error {
	if nodeType != config.ENTER_PRICE_TYPE {
		projectId = config.BaseLoad().UcodeNamespace
	}
	return s.pool[projectId].SetEX(ctx, key, value, duration).Err()
}

func (s Storage) Get(ctx context.Context, key string, projectId string, nodeType string) (string, error) {
	if nodeType != config.ENTER_PRICE_TYPE {
		projectId = config.BaseLoad().UcodeNamespace
	}
	return s.pool[projectId].Get(ctx, key).Result()
}

func (s Storage) Del(ctx context.Context, keys string, projectId string, nodeType string) error {
	if nodeType != config.ENTER_PRICE_TYPE {
		projectId = config.BaseLoad().UcodeNamespace
	}
	return s.pool[projectId].Del(ctx, keys).Err()
}

func (s Storage) Set(ctx context.Context, key string, value any, duration time.Duration, projectId string, nodeType string) error {
	if nodeType != config.ENTER_PRICE_TYPE {
		projectId = config.BaseLoad().UcodeNamespace
	}
	return s.pool[projectId].Set(ctx, key, value, duration).Err()
}

func (s Storage) DelMany(ctx context.Context, keys []string, projectId string, nodeType string) error {
	if nodeType != config.ENTER_PRICE_TYPE {
		projectId = config.BaseLoad().UcodeNamespace
	}

	return s.pool[projectId].Del(ctx, keys...).Err()
}
