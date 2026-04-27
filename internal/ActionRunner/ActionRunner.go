package ActionRunner

import (
	"context"
	"log/slog"
	"sync"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
)

type ActionArgs struct {
	Logger *slog.Logger
	Action ActionDescriptor
}

type ActionRunner struct {
	ch        chan ActionArgs
	wg        *sync.WaitGroup
	ctx       context.Context
	cancel    func()
	actionsDB *actiondb.ActionDB
}

// actionChanBufferSize controls how many webhook dispatches can queue before
// HTTP handlers block waiting for the runner to consume them.
const actionChanBufferSize = 10

func New(ctx context.Context, actionsDB *actiondb.ActionDB) (runner ActionRunner) {
	runner.ch = make(chan ActionArgs, actionChanBufferSize)
	runner.ctx, runner.cancel = context.WithCancel(ctx)
	runner.wg = &sync.WaitGroup{}
	runner.actionsDB = actionsDB
	go runner.listen()
	return runner
}

func (runner ActionRunner) Chan() chan ActionArgs {
	return runner.ch
}

func (runner ActionRunner) Cancel() {
	runner.cancel()
}

func (runner ActionRunner) Done() <-chan struct{} {
	return runner.ctx.Done()
}

func (runner ActionRunner) Wait() {
	runner.wg.Wait()
	runner.cancel()
}

func (runner ActionRunner) listen() {
	for {
		select {
		case <-runner.ctx.Done():
			return
		case args := <-runner.ch:
			runner.wg.Go(func() {
				runner.executeAction(args.Logger, args.Action)
			})
		}
	}
}
