package actionrunner

import (
	"context"
	"log/slog"
	"sync"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
)

type ActionArgs struct {
	Logger     *slog.Logger
	Action     ActionDescriptor
	DeliveryID string
}

type ActionRunner struct {
	ch        chan ActionArgs
	wg        *sync.WaitGroup
	ctx       context.Context
	cancel    func()
	actionsDB *actionsdb.ActionDB
}

// actionChanBufferSize controls how many webhook dispatches can queue before
// HTTP handlers block waiting for the runner to consume them.
const actionChanBufferSize = 10

func New(ctx context.Context, actionsDB *actionsdb.ActionDB) (r ActionRunner) {
	r.ch = make(chan ActionArgs, actionChanBufferSize)
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.wg = &sync.WaitGroup{}
	r.actionsDB = actionsDB
	go r.listen()
	return r
}

func (r ActionRunner) Chan() chan ActionArgs {
	return r.ch
}

func (r ActionRunner) Cancel() {
	r.cancel()
}

func (r ActionRunner) Done() <-chan struct{} {
	return r.ctx.Done()
}

func (r ActionRunner) Wait() {
	r.wg.Wait()
	r.cancel()
}

func (r ActionRunner) listen() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case args := <-r.ch:
			r.wg.Go(func() {
				r.executeAction(args.Logger, args.Action, args.DeliveryID)
			})
		}
	}
}
