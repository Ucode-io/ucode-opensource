package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/stretchr/testify/require"
)

// discardLogger keeps the rollback's own diagnostics out of the test output.
type discardLogger struct{}

func (discardLogger) Debug(string, ...logger.Field)  {}
func (discardLogger) Info(string, ...logger.Field)   {}
func (discardLogger) Warn(string, ...logger.Field)   {}
func (discardLogger) Error(string, ...logger.Field)  {}
func (discardLogger) DPanic(string, ...logger.Field) {}
func (discardLogger) Panic(string, ...logger.Field)  {}
func (discardLogger) Fatal(string, ...logger.Field)  {}

func newTestRollback() *registerRollback {
	return &registerRollback{log: discardLogger{}}
}

func TestRegisterRollbackUndoesInReverseOrder(t *testing.T) {
	var undone []string

	r := newTestRollback()
	for _, name := range []string{"company", "project", "environment"} {
		r.record(name, func(context.Context) error {
			undone = append(undone, name)
			return nil
		})
	}

	r.run(context.Background(), errors.New("boom"))

	// Reverse order is the whole point: the project references the company, so
	// undoing the company first would fail on the foreign key.
	require.Equal(t, []string{"environment", "project", "company"}, undone)
}

func TestRegisterRollbackContinuesAfterAFailedStep(t *testing.T) {
	var undone []string

	r := newTestRollback()
	r.record("company", func(context.Context) error {
		undone = append(undone, "company")
		return nil
	})
	r.record("project", func(context.Context) error {
		return errors.New("company-service is unreachable")
	})
	r.record("environment", func(context.Context) error {
		undone = append(undone, "environment")
		return nil
	})

	r.run(context.Background(), errors.New("boom"))

	// A step that cannot be undone must not strand the ones below it — above
	// all the company row, which is what blocks a retry.
	require.Equal(t, []string{"environment", "company"}, undone)
}

func TestRegisterRollbackOutlivesACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the caller hung up, which is often what failed the registration

	var (
		ran      bool
		undoErr  error
		deadline time.Time
		hasDue   bool
	)

	r := newTestRollback()
	r.record("company", func(ctx context.Context) error {
		ran = true
		undoErr = ctx.Err()
		deadline, hasDue = ctx.Deadline()
		return nil
	})

	r.run(ctx, errors.New("boom"))

	require.True(t, ran, "the undo must run even though the request context is done")
	// Not merely reached: reached with a live context. Deriving the
	// compensation from the request context would hand every undo a context
	// that is already cancelled, and the gRPC calls would fail on arrival.
	require.NoError(t, undoErr, "the undo must not inherit the caller's cancellation")
	require.True(t, hasDue, "the undo must still be bounded by its own deadline")
	require.WithinDuration(t, time.Now().Add(rollbackTimeout), deadline, time.Second)
}

func TestRegisterRollbackDoesNothingWithoutSteps(t *testing.T) {
	// A registration that fails before its first write has nothing to undo,
	// and must not spend the rollback timeout discovering that.
	r := newTestRollback()
	require.NotPanics(t, func() { r.run(context.Background(), errors.New("boom")) })
}
