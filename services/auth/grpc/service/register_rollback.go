package service

import (
	"context"
	"time"

	"github.com/saidamir98/udevs_pkg/logger"
)

// registerRollback undoes the writes a company registration made before it
// failed.
//
// Creating the first company is nine writes spread over two services and two
// databases: the company, its project, the environment, an api key, the admin
// user, that user's membership of the project, the project's own Postgres
// database and its registration on object-builder. No transaction spans them,
// and none easily could — they are separate services reached over gRPC.
//
// What can be had instead is atomicity as the caller sees it. Each step
// records how to undo itself; if a later one fails, the recorded undos run in
// reverse and the installation is left as it was found.
//
// The company row is why this matters. While one exists, company-service
// refuses to create another, so a half-finished registration turned every
// retry into "only one company allowed" — a refusal indistinguishable from an
// installation that was already set up, which is exactly how it was read. The
// user was told ucode was running and met a login that failed.
type registerRollback struct {
	log   logger.LoggerI
	steps []rollbackStep
}

type rollbackStep struct {
	what string
	undo func(context.Context) error
}

// rollbackTimeout bounds the compensation as a whole. It is generous because
// dropping the project's database is the slow part, and short enough that a
// wedged dependency cannot hold the request open indefinitely.
const rollbackTimeout = 30 * time.Second

// record appends an undo for a write that has just succeeded. Order matters:
// run replays them in reverse, so record them in the order they happen.
func (r *registerRollback) record(what string, undo func(context.Context) error) {
	r.steps = append(r.steps, rollbackStep{what: what, undo: undo})
}

// run undoes every recorded step, most recent first.
//
// Failures here are logged, not returned: the caller is already failing, and
// the error it reports should say what went wrong with the registration, not
// what went wrong cleaning up after it. One undo failing does not stop the
// rest — stopping would leave more behind than carrying on.
func (r *registerRollback) run(ctx context.Context, cause error) {
	if len(r.steps) == 0 {
		return
	}

	// By the time we are here the request's context is often already done —
	// the caller timed out or hung up, and that is frequently what failed the
	// registration in the first place. Compensation has to outlive it.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
	defer cancel()

	r.log.Error("---RegisterCompany--->rolling back",
		logger.Error(cause), logger.Int("steps", len(r.steps)))

	for i := len(r.steps) - 1; i >= 0; i-- {
		step := r.steps[i]
		if err := step.undo(ctx); err != nil {
			r.log.Error("---RegisterCompany--->rollback step failed",
				logger.String("step", step.what), logger.Error(err))
			continue
		}
		r.log.Info("---RegisterCompany--->rolled back", logger.String("step", step.what))
	}
}
