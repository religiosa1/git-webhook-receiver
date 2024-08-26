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
	actionsDb *actiondb.ActionDb
}

func New(ctx context.Context, actionsDb *actiondb.ActionDb) (runner ActionRunner) {
	runner.ch = make(chan ActionArgs)
	runner.ctx, runner.cancel = context.WithCancel(ctx)
	runner.wg = &sync.WaitGroup{}
	runner.actionsDb = actionsDb
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
			runner.wg.Add(1)
			go func() {
				defer runner.wg.Done()
				runner.executeAction(args.Logger, args.Action)
			}()
		}
	}
}
