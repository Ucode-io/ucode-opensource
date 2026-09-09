package apilimits

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/go-redis/redis/v8"
)

// Tracker counts incoming API calls in an L1 in-memory cache and flushes
// totals to Redis on a regular interval. Safe for concurrent middleware use.
type Tracker struct {
	worker
	flushInterval time.Duration
	rdb           *redis.Client
	l1Cache       sync.Map
}

func NewTracker(rdb *redis.Client, flushInterval time.Duration) *Tracker {
	return &Tracker{
		flushInterval: flushInterval,
		rdb:           rdb,
	}
}

func (t *Tracker) Start(ctx context.Context) {
	t.spawn(ctx, t.run)
	log.Printf("[Tracker] started flush_interval=%v", t.flushInterval)
}

func (t *Tracker) Stop() {
	t.shutdown()
	t.flush() // final drain on graceful shutdown
	log.Println("[Tracker] stopped")
}

func (t *Tracker) run(ctx context.Context) {
	ticker := time.NewTicker(t.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.flush()
		}
	}
}

func (t *Tracker) flush() {
	ctx := context.Background()
	now := time.Now()
	pipe := t.rdb.Pipeline()
	hasData := false

	t.l1Cache.Range(func(k, v any) bool {
		projectID := k.(string)
		delta := v.(*atomic.Int64).Swap(0)
		if delta == 0 {
			return true
		}
		hasData = true

		minKey := fmt.Sprintf(config.KeyRateMin, projectID, now.Format("2006-01-02-15-04"))
		hourKey := fmt.Sprintf(config.KeyRateHour, projectID, now.Format("2006-01-02-15"))
		dayKey := fmt.Sprintf(config.KeyRateDay, projectID, now.Format("2006-01-02"))

		pipe.IncrBy(ctx, minKey, delta)
		pipe.Expire(ctx, minKey, 15*time.Minute)
		pipe.IncrBy(ctx, hourKey, delta)
		pipe.Expire(ctx, hourKey, 24*time.Hour)
		pipe.IncrBy(ctx, dayKey, delta)
		pipe.Expire(ctx, dayKey, 48*time.Hour)
		pipe.HIncrBy(ctx, fmt.Sprintf(config.KeyUsagePending, projectID), config.KeyUsageTotalField, delta)
		return true
	})

	if hasData {
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("[Tracker] flush error: %v", err)
		}
	}
}
