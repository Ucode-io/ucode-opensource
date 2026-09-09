package cron

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	pb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/company_service"

	"github.com/robfig/cron/v3"
)

type TaskScheduler struct {
	cronJob *cron.Cron
	logger  logger.LoggerI
	storage storage.StorageI
	svcs    client.ServiceManagerI
}

type TaskSchedulerI interface {
	RunJobs(context.Context) error
	DeleteFunctionLogs(context.Context) error
	RotateVersionHistoryPartitions(context.Context) error
}

func New(log logger.LoggerI, storage storage.StorageI, svcs client.ServiceManagerI) TaskSchedulerI {
	var cronJob = cron.New()
	defer cronJob.Start()
	return &TaskScheduler{
		cronJob: cronJob,
		logger:  log,
		storage: storage,
		svcs:    svcs,
	}
}

func (t *TaskScheduler) RunJobs(ctx context.Context) error {
	t.logger.Info("Jobs Started:")

	if _, err := t.cronJob.AddFunc("0 0 * * *", func() {
		if err := t.DeleteFunctionLogs(ctx); err != nil {
			t.logger.Error("error in DeleteFunctionLogs", logger.Error(err))
		}
	}); err != nil {
		return err
	}

	if _, err := t.cronJob.AddFunc("0 1 * * *", func() {
		if err := t.RotateVersionHistoryPartitions(ctx); err != nil {
			t.logger.Error("error in RotateVersionHistoryPartitions", logger.Error(err))
		}
	}); err != nil {
		return err
	}

	return nil
}

func (t *TaskScheduler) DeleteFunctionLogs(ctx context.Context) error {
	t.logger.Info("Running DeleteFunctionLogs job ...")

	// TODO pagination qilib ishlidgan qilish kere, bitta kop daniyla kelib qolishi mumkun

	response, err := t.svcs.ResourceService().GetListResourceEnvironment(ctx, &company_service.GetListResourceEnvironmentReq{
		ResourceType: pb.ResourceType_POSTGRESQL,
	})
	if err != nil {
		t.logger.Info("error in getting resource environment", logger.Error(err))
		return err
	}

	for i := range response.Data {
		err = t.storage.VersionHistory().DeleteFunctionLogs(ctx, response.Data[i].Id)
		if err != nil {
			t.logger.Info("error in deleting function logs", logger.Error(err))
			continue
		}
	}

	return nil
}

// RotateVersionHistoryPartitions walks every tenant Postgres pool and:
//  - creates the partition for the next week (idempotent),
//  - drops partitions whose date range is fully outside the retention window.
// One failing tenant must not stop the rotation for the rest.
func (t *TaskScheduler) RotateVersionHistoryPartitions(ctx context.Context) error {
	t.logger.Info("Running RotateVersionHistoryPartitions job ...")

	response, err := t.svcs.ResourceService().GetListResourceEnvironment(ctx, &company_service.GetListResourceEnvironmentReq{
		ResourceType: pb.ResourceType_POSTGRESQL,
	})
	if err != nil {
		t.logger.Info("error in getting resource environment", logger.Error(err))
		return err
	}

	for i := range response.Data {
		if err := t.storage.VersionHistory().RotateVersionHistoryPartitions(ctx, response.Data[i].Id); err != nil {
			t.logger.Error("error in rotating version_history partitions",
				logger.String("project_id", response.Data[i].Id),
				logger.Error(err),
			)
			continue
		}
	}

	return nil
}
