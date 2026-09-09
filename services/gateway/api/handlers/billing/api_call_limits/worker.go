package apilimits

import (
	"context"
	"sync"
)

const (
	redisScanBatch       = 100
	usageLogTimeRangeSec = 3600
)

// worker provides the shared Start/Stop lifecycle used by all background workers.
// Embed it in any struct that runs a single background goroutine.
type worker struct {
	wg       sync.WaitGroup
	cancelFn context.CancelFunc
}

func (w *worker) spawn(ctx context.Context, fn func(context.Context)) {
	ctx, w.cancelFn = context.WithCancel(ctx)
	w.wg.Add(1)
	go func() { defer w.wg.Done(); fn(ctx) }()
}

func (w *worker) shutdown() {
	if w.cancelFn != nil {
		w.cancelFn()
	}
	w.wg.Wait()
}