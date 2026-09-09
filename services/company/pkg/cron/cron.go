package cron

import (
	"context"
	"errors"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"github.com/robfig/cron/v3"
)

type TaskScheduler struct {
	cron     *cron.Cron
	log      logger.Logger
	strg     storage.StorageI
	services client.ServiceManagerI
	cfg      config.BaseConfig
}

type TaskSchedulerI interface {
	RunJobs(context.Context) error
}

func New(log logger.Logger, storage storage.StorageI, services client.ServiceManagerI, cfg config.BaseConfig) TaskSchedulerI {
	// SkipIfStillRunning serialises each job with itself: if a renewals tick is
	// still running when the next one fires, the new run is skipped. Combined with
	// the per-row FOR UPDATE locks in the processors, this keeps renewals idempotent.
	cron := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	defer cron.Start()
	return &TaskScheduler{
		cron:     cron,
		log:      log,
		strg:     storage,
		services: services,
		cfg:      cfg,
	}
}

func (t *TaskScheduler) RunJobs(ctx context.Context) error {
	t.log.Info("Jobs Started:")
	_, _ = t.cron.AddFunc("*/2 * * * *", func() { // every 2 minutes
		t.ProcessRenewals(ctx)
	})

	_, _ = t.cron.AddFunc("0 0 * * *", func() { // daily at midnight
		t.DeactivateDeadProjects(ctx)
	})

	_, _ = t.cron.AddFunc("*/5 * * * *", func() { // every 5 minutes
		t.ReconcileIpakPayments(ctx)
	})

	return nil
}

func (t *TaskScheduler) DeactivateDeadProjects(ctx context.Context) {
	t.log.Info("Started dead-project deactivation job.....")

	if err := t.strg.Billing().DeactivateDeadProjects(ctx); err != nil {
		t.log.Error("Error in deactivating dead projects", logger.Error(err))
		return
	}

	t.log.Info("Finished dead-project deactivation job.....")
}

func (t *TaskScheduler) ProcessRenewals(ctx context.Context) {
	t.log.Info("Started process renewals job.....")

	ucodeErr := t.strg.Billing().ProcessUcodeRenewals(ctx)
	if ucodeErr != nil {
		t.log.Error("Error in processing ucode renewals", logger.Error(ucodeErr))
	}

	ugenErr := t.strg.Billing().ProcessUgenRenewals(ctx)
	if ugenErr != nil {
		t.log.Error("Error in processing ugen renewals", logger.Error(ugenErr))
	}

	userSeatErr := t.strg.Billing().ProcessUserSeatRenewals(ctx, t.getProjectUsersCount)
	if userSeatErr != nil {
		t.log.Error("Error in processing user seat renewals", logger.Error(userSeatErr))
	}

	if ucodeErr != nil || ugenErr != nil || userSeatErr != nil {
		t.log.Info("Finished process renewals job with errors.....")
		return
	}

	t.log.Info("Finished process renewals job.....")
}

func (t *TaskScheduler) getProjectUsersCount(ctx context.Context, projectID string) (int32, error) {
	if t.services == nil || t.services.UserService() == nil {
		return 0, errors.New("auth user service is not configured")
	}

	resp, err := t.services.UserService().GetProjectUsersCount(ctx, &auth_service.GetProjectUsersCountRequest{
		ProjectId: projectID,
	})
	if err != nil {
		return 0, err
	}

	return resp.GetCount(), nil
}

//func (t *TaskScheduler) ProjectsInactivation(ctx context.Context) {
//	t.log.Info("Started projects inactivation job.....")
//	err := t.strg.Billing().InactivateProjects(ctx)
//	if err != nil {
//		t.log.Error("Error in updating projects status", logger.Error(err))
//		return
//	}
//
//	t.log.Info("Finished projects inactivation job.....")
//}
