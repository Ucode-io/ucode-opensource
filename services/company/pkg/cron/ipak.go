package cron

import (
	"context"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
)

// ipakReconcileMinAge is how long a pending Ipak top-up must sit before the cron
// re-checks it. The bank callback fires once (no retries), so this sweep is the
// safety net that settles payments whose callback was missed or whose customer
// closed the tab before the return page could poll.
const ipakReconcileMinAge = 2 * time.Minute

// ReconcileIpakPayments re-checks pending Ipak Yo'li top-ups against the bank via
// the shared, idempotent ConfirmIpakPayment path. Because that path is guarded in
// storage, re-confirming a payment already settled by the callback or the frontend
// poll is a no-op.
func (t *TaskScheduler) ReconcileIpakPayments(ctx context.Context) {
	if t.cfg.IpakEcommAccessToken == "" || t.cfg.IpakEcommBaseUrl == "" {
		return // Ipak not configured (e.g. bank hasn't issued the cashbox token yet).
	}

	pending, err := t.strg.Billing().ListPendingIpakTransactions(ctx, ipakReconcileMinAge)
	if err != nil {
		t.log.Error("Error listing pending ipak transactions", logger.Error(err))
		return
	}
	if len(pending) == 0 {
		return
	}

	t.log.Info("Started ipak reconcile job.....")
	billing := service.NewBillingService(t.strg, t.log, t.cfg)
	for _, txn := range pending {
		if txn.GetExternalId() == "" {
			continue
		}
		if _, err := billing.ConfirmIpakPayment(ctx, &pb.ConfirmIpakPaymentRequest{TransferId: txn.GetExternalId()}); err != nil {
			t.log.Error("Error reconciling ipak transaction", logger.Error(err))
		}
	}
	t.log.Info("Finished ipak reconcile job.....")
}
