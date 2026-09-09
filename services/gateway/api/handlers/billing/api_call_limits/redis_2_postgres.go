package apilimits

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/go-redis/redis/v8"
)

// MetricsConsumer drains pending API usage counts from Redis and logs them to
// the billing service (PostgreSQL via gRPC) on a regular interval.
type MetricsConsumer struct {
	worker
	rdb            *redis.Client
	companyService services.CompanyServiceI
	flushInterval  time.Duration
}

func NewMetricsConsumer(rdb *redis.Client, companyService services.CompanyServiceI, flushInterval time.Duration) *MetricsConsumer {
	return &MetricsConsumer{
		rdb:            rdb,
		companyService: companyService,
		flushInterval:  flushInterval,
	}
}

func (c *MetricsConsumer) Start(ctx context.Context) {
	c.spawn(ctx, c.run)
	log.Printf("[MetricsConsumer] started db_flush_interval=%v", c.flushInterval)
}

func (c *MetricsConsumer) Stop() {
	c.shutdown()
	log.Println("[MetricsConsumer] stopped")
}

func (c *MetricsConsumer) run(ctx context.Context) {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.flushApiUsage(ctx)
		}
	}
}

func (c *MetricsConsumer) flushApiUsage(ctx context.Context) {
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, config.KeyUsagePendingPattern, redisScanBatch).Result()
		if err != nil {
			log.Printf("[MetricsConsumer] SCAN error: %v", err)
			return
		}

		for _, key := range keys {
			projectID := key[len(config.KeyUsagePendingPrefix):]
			c.logProjectUsage(ctx, key, projectID)
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}
}

func (c *MetricsConsumer) logProjectUsage(ctx context.Context, key, projectID string) {
	countStr, err := c.rdb.HGet(ctx, key, config.KeyUsageTotalField).Result()
	if err != nil {
		return
	}
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil || count <= 0 {
		return
	}

	_, err = c.companyService.Billing().LogUsage(ctx, &pb.LogUsageRequest{
		ProjectId: projectID,
		Count:     count,
		TimeRange: usageLogTimeRangeSec,
		DateTime:  time.Now().Truncate(time.Hour).Format("2006-01-02 15:00:00"),
	})
	if err != nil {
		// Leave count in Redis — will retry on next tick.
		log.Printf("[MetricsConsumer] LogUsage project=%s: %v", projectID, err)
		return
	}

	if err = c.rdb.HIncrBy(ctx, key, config.KeyUsageTotalField, -count).Err(); err != nil {
		log.Printf("[MetricsConsumer] HIncrBy project=%s: %v", projectID, err)
	}
}