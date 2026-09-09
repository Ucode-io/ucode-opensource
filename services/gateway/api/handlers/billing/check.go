package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/go-redis/redis/v8"
)

var ErrAssetLimitExceeded = errors.New("you have reached the asset size limit on your current plan. Please upgrade to upload more files")

func CheckAssetSizeLimit(ctx context.Context, companyServices services.CompanyServiceI, srvc services.ServiceManagerI, projectId, resourceEnvId string, newFileSizeBytes int64) error {
	type (
		projRes struct {
			fareId string
			err    error
		}
		usageRes struct {
			assetBytes int64
			err        error
		}
	)

	projCh := make(chan projRes, 1)
	usageCh := make(chan usageRes, 1)

	go func() {
		project, err := companyServices.Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId})
		if err != nil {
			projCh <- projRes{err: err}
			return
		}
		projCh <- projRes{fareId: project.GetFareId()}
	}()

	go func() {
		usage, err := srvc.GoObjectBuilderService().ObjectBuilder().GetResourceUsage(ctx, &nb.GetResourceUsageRequest{ProjectId: resourceEnvId})
		if err != nil {
			usageCh <- usageRes{err: err}
			return
		}
		usageCh <- usageRes{assetBytes: usage.GetAssetSize()}
	}()

	pRes := <-projCh
	if pRes.err != nil {
		return pRes.err
	}

	if len(pRes.fareId) == 0 {
		return nil
	}

	uRes := <-usageCh
	if uRes.err != nil {
		return uRes.err
	}

	totalMB := int32((uRes.assetBytes + newFileSizeBytes) / 1024 / 1024)

	log.Printf("[billing] CheckAssetSizeLimit: projectId=%s fareId=%s assetBytes=%d newFileBytes=%d totalMB=%d",
		projectId, pRes.fareId, uRes.assetBytes, newFileSizeBytes, totalMB)

	limitResp, err := companyServices.Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
		Type:   config.FARE_ASSET_SIZE,
		FareId: pRes.fareId,
		Count:  totalMB,
	})
	if err != nil {
		log.Printf("[billing] CheckAssetSizeLimit: CompareFunction error: %v", err)
		return err
	}

	log.Printf("[billing] CheckAssetSizeLimit: hasAccess=%v", limitResp.GetHasAccess())

	if !limitResp.GetHasAccess() {
		return ErrAssetLimitExceeded
	}

	return nil
}

// ─── Database size limit ───────────────────────────────────────────────────

var ErrDatabaseLimitExceeded = errors.New("your database size limit has been reached on your current plan. Please upgrade to add more data")

// CacheEntry is the billing context persisted per project in Redis.
// Shared with BillingLimitWorker so it can refresh limits without extra DB calls.
type CacheEntry struct {
	EnvId    string `json:"e"`
	FareId   string `json:"f"`
	NodeType string `json:"n"`
}

func CheckDatabaseLimit(ctx context.Context, rdb *redis.Client, companyServices services.CompanyServiceI, srvc services.ServiceManagerI, projectId, resourceEnvId, nodeType string) error {
	limitKey := fmt.Sprintf(config.KeyBillingDbLimit, projectId)

	if val, err := rdb.Get(ctx, limitKey).Result(); err == nil {
		if val == "0" {
			return ErrDatabaseLimitExceeded
		}
		return nil // "1" = allowed
	}

	type (
		projRes struct {
			fareId string
			err    error
		}
		usageRes struct {
			dbSizeMB int32
			err      error
		}
	)

	projCh := make(chan projRes, 1)
	usageCh := make(chan usageRes, 1)

	go func() {
		project, err := companyServices.Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId})
		if err != nil {
			projCh <- projRes{err: err}
			return
		}
		projCh <- projRes{fareId: project.GetFareId()}
	}()

	go func() {
		usage, err := srvc.GoObjectBuilderService().ObjectBuilder().GetResourceUsage(
			ctx, &nb.GetResourceUsageRequest{ProjectId: resourceEnvId},
		)
		if err != nil {
			usageCh <- usageRes{err: err}
			return
		}
		usageCh <- usageRes{dbSizeMB: int32(usage.GetDatabaseSize() / 1024 / 1024)}
	}()

	pRes := <-projCh
	if pRes.err != nil {
		return pRes.err
	}

	if pRes.fareId == "" {
		storeBillingCache(ctx, rdb, projectId, resourceEnvId, nodeType, pRes.fareId, true)
		return nil
	}

	uRes := <-usageCh
	if uRes.err != nil {
		return uRes.err
	}

	limitResp, err := companyServices.Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
		Type:   config.FARE_DATABASE_SIZE,
		FareId: pRes.fareId,
		Count:  uRes.dbSizeMB,
	})
	if err != nil {
		return err
	}

	allowed := limitResp.GetHasAccess()
	storeBillingCache(ctx, rdb, projectId, resourceEnvId, nodeType, pRes.fareId, allowed)

	if !allowed {
		return ErrDatabaseLimitExceeded
	}
	return nil
}

func storeBillingCache(ctx context.Context, rdb *redis.Client, projectId, resourceEnvId, nodeType, fareId string, allowed bool) {
	limitKey := fmt.Sprintf(config.KeyBillingDbLimit, projectId)
	ctxKey := fmt.Sprintf(config.KeyBillingDbCtx, projectId)

	val := "1"
	if !allowed {
		val = "0"
	}

	ctxData, err := json.Marshal(CacheEntry{EnvId: resourceEnvId, FareId: fareId, NodeType: nodeType})
	if err != nil {
		return
	}

	pipe := rdb.Pipeline()
	pipe.Set(ctx, limitKey, val, 15*time.Minute)
	pipe.Set(ctx, ctxKey, string(ctxData), 30*time.Minute)
	pipe.Exec(ctx)
}