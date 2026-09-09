package apilimits

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/billing"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/go-redis/redis/v8"
)

// BillingLimitWorker periodically refreshes per-project billing limit flags in
// Redis so that BillingLimitMiddleware can enforce limits without a DB round-trip.
type BillingLimitWorker struct {
	worker
	rdb            *redis.Client
	serviceNodes   services.ServiceNodesI
	companyService services.CompanyServiceI
	ucodeNamespace string
	flushInterval  time.Duration
}

func NewBillingLimitWorker(rdb *redis.Client, serviceNodes services.ServiceNodesI, companyService services.CompanyServiceI, ucodeNamespace string, flushInterval time.Duration) *BillingLimitWorker {
	return &BillingLimitWorker{
		rdb:            rdb,
		serviceNodes:   serviceNodes,
		companyService: companyService,
		ucodeNamespace: ucodeNamespace,
		flushInterval:  flushInterval,
	}
}

func (w *BillingLimitWorker) Start(ctx context.Context) {
	w.spawn(ctx, w.run)
	log.Printf("[BillingLimitWorker] started refresh_interval=%v", w.flushInterval)
}

func (w *BillingLimitWorker) Stop() {
	w.shutdown()
	log.Println("[BillingLimitWorker] stopped")
}

func (w *BillingLimitWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.refreshAll(ctx)
			w.refreshApiLimits(ctx)
		}
	}
}

func (w *BillingLimitWorker) refreshAll(ctx context.Context) {
	var cursor uint64
	for {
		keys, next, err := w.rdb.Scan(ctx, cursor, config.KeyBillingDbLimitPattern, redisScanBatch).Result()
		if err != nil {
			log.Printf("[BillingLimitWorker] SCAN error: %v", err)
			return
		}
		for _, key := range keys {
			w.refreshProject(ctx, key[len(config.KeyBillingDbLimitPrefix):])
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
}

func (w *BillingLimitWorker) refreshProject(ctx context.Context, projectId string) {
	raw, err := w.rdb.Get(ctx, fmt.Sprintf(config.KeyBillingDbCtx, projectId)).Result()
	if err != nil {
		return // context not yet written — skip until first live check
	}

	var entry billing.CacheEntry
	if err = json.Unmarshal([]byte(raw), &entry); err != nil || entry.EnvId == "" || entry.FareId == "" {
		return
	}

	namespace := w.ucodeNamespace
	if entry.NodeType == config.ENTER_PRICE_TYPE {
		namespace = projectId
	}

	srvc, err := w.serviceNodes.Get(namespace)
	if err != nil {
		log.Printf("[BillingLimitWorker] service not found namespace=%s project=%s: %v", namespace, projectId, err)
		return
	}

	usage, err := srvc.GoObjectBuilderService().ObjectBuilder().GetResourceUsage(
		ctx, &nb.GetResourceUsageRequest{ProjectId: entry.EnvId},
	)
	if err != nil {
		log.Printf("[BillingLimitWorker] GetResourceUsage project=%s: %v", projectId, err)
		return
	}

	limitResp, err := w.companyService.Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
		Type:   config.FARE_DATABASE_SIZE,
		FareId: entry.FareId,
		Count:  int32(usage.GetDatabaseSize() / 1024 / 1024),
	})
	if err != nil {
		log.Printf("[BillingLimitWorker] CompareFunction project=%s: %v", projectId, err)
		return
	}

	w.setBillingFlag(ctx, fmt.Sprintf(config.KeyBillingDbLimit, projectId), limitResp.GetHasAccess(), 15*time.Minute)
}

func (w *BillingLimitWorker) refreshApiLimits(ctx context.Context) {
	var cursor uint64
	for {
		keys, next, err := w.rdb.Scan(ctx, cursor, config.KeyUsagePendingPattern, redisScanBatch).Result()
		if err != nil {
			log.Printf("[BillingLimitWorker] refreshApiLimits SCAN error: %v", err)
			return
		}
		for _, key := range keys {
			w.refreshApiLimit(ctx, key[len(config.KeyUsagePendingPrefix):])
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
}

func (w *BillingLimitWorker) refreshApiLimit(ctx context.Context, projectId string) {
	fareId, err := w.getFareId(ctx, projectId)
	if err != nil || fareId == "" {
		return
	}

	metricsResp, err := w.companyService.Billing().GetApiCallMonitoringMetrics(
		ctx, &pb.GetApiCallMonitoringMetricsRequest{ProjectId: projectId},
	)
	if err != nil {
		log.Printf("[BillingLimitWorker] GetApiCallMonitoringMetrics project=%s: %v", projectId, err)
		return
	}

	totalCalls := metricsResp.GetTotalMonthlyCalls()
	if totalCalls == 0 {
		// No recorded calls — clearly within limit.
		w.setBillingFlag(ctx, fmt.Sprintf(config.KeyBillingApiLimit, projectId), true, 15*time.Minute)
		return
	}

	limitResp, err := w.companyService.Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
		Type:   config.FARE_REQUEST_PER_MONTH,
		FareId: fareId,
		Count:  int32(totalCalls),
	})
	if err != nil {
		log.Printf("[BillingLimitWorker] CompareFunction(api) project=%s: %v", projectId, err)
		return
	}

	w.setBillingFlag(ctx, fmt.Sprintf(config.KeyBillingApiLimit, projectId), limitResp.GetHasAccess(), 15*time.Minute)
}

func (w *BillingLimitWorker) getFareId(ctx context.Context, projectId string) (string, error) {
	fareKey := fmt.Sprintf(config.KeyBillingFareId, projectId)
	if cached, err := w.rdb.Get(ctx, fareKey).Result(); err == nil && cached != "" {
		return cached, nil
	}

	proj, err := w.companyService.Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId})
	if err != nil {
		return "", err
	}

	fareId := proj.GetFareId()
	if fareId != "" {
		w.rdb.Set(ctx, fareKey, fareId, 30*time.Minute)
	}
	return fareId, nil
}

// setBillingFlag writes "1" (allowed) or "0" (blocked) to the given Redis key.
func (w *BillingLimitWorker) setBillingFlag(ctx context.Context, key string, allowed bool, ttl time.Duration) {
	val := "1"
	if !allowed {
		val = "0"
	}
	if err := w.rdb.Set(ctx, key, val, ttl).Err(); err != nil {
		log.Printf("[BillingLimitWorker] SET %s: %v", key, err)
	}
}
