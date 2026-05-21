package actionrunner

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
)

// ActionArgs are arguments passed to run an action/pipeline by an ActionRunner
type ActionArgs struct {
	Logger     *slog.Logger
	Action     ActionDescriptor
	DeliveryID string
}

type ActionRunner struct {
	ch           chan ActionArgs
	wg           *sync.WaitGroup
	ctx          context.Context
	cancel       func()
	actionsDB    *actionsdb.ActionDB
	tmpOutputMgr tmpoutput.Manager
}

// actionChanBufferSize controls how many webhook dispatches can queue before
// HTTP handlers block waiting for the runner to consume them.
const actionChanBufferSize = 10

func New(ctx context.Context, actionsDB *actionsdb.ActionDB, tmpOutputMgr tmpoutput.Manager) *ActionRunner {
	ctx, cancel := context.WithCancel(ctx)
	r := ActionRunner{
		ch:           make(chan ActionArgs, actionChanBufferSize),
		wg:           &sync.WaitGroup{},
		actionsDB:    actionsDB,
		ctx:          ctx,
		cancel:       cancel,
		tmpOutputMgr: tmpOutputMgr,
	}
	go r.listen()
	return &r
}

func (r *ActionRunner) Chan() chan ActionArgs {
	return r.ch
}

func (r *ActionRunner) Cancel() {
	r.cancel()
}

func (r *ActionRunner) Done() <-chan struct{} {
	return r.ctx.Done()
}

func (r *ActionRunner) Wait() {
	r.wg.Wait()
	r.cancel()
}

func (r *ActionRunner) listen() {
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

//------------------------------------------------------------------------------
// Private parts

func (r *ActionRunner) executeAction(
	logger *slog.Logger,
	actionDescriptor ActionDescriptor,
	deliveryID string,
) {
	action := actionDescriptor.Action
	pipeLogger := logger.With(slog.String("pipeId", actionDescriptor.PipeID))
	pipeLogger.Info("Running action", slog.Int("action_index", actionDescriptor.Index))

	if r.actionsDB != nil {
		err := r.actionsDB.CreateRecord(actionDescriptor.PipeID, actionDescriptor.Project, deliveryID, action)
		if err != nil {
			pipeLogger.Error("Error creating pipeline record in the db", slog.Any("error", err))
			return
		}
	}

	outputWriter, err := r.tmpOutputMgr.Create(actionDescriptor.PipeID)
	if err != nil {
		pipeLogger.Error("Error creating temporary file to capture action's output", slog.Any("error", err))
		return
	}
	defer func() {
		err := r.tmpOutputMgr.Close(actionDescriptor.PipeID)
		if err != nil {
			pipeLogger.Error("Error closing action output", slog.Any("error", err))
		}
	}()

	sysProcAttr, err := getSysProcAttr(action.User)
	if err != nil {
		logger.Error("Error creating process attributes for action", slog.Any("error", err))
		return
	}

	if action.User != "" {
		logger.Debug("Running from a user", slog.String("user", action.User))
	}

	actionCtx, cancelAction := context.WithTimeout(r.ctx, action.Timeout)
	defer cancelAction()

	var actionErr error
	if len(action.Run) > 0 {
		logger.Debug("Running the command", slog.Any("command", action.Run))
		actionErr = executeActionRun(actionCtx, action, sysProcAttr, outputWriter)
	} else {
		logger.Debug("Running the script", slog.String("script", action.Script))
		actionErr = executeActionScript(actionCtx, action, sysProcAttr, outputWriter)
	}

	if actionErr != nil {
		logger.Error("Error while running the action", slog.Any("error", actionErr))
	} else {
		logger.Info("Action successfully finished")
	}

	if r.actionsDB != nil {
		var outputForDB []byte
		outputReader, err := r.tmpOutputMgr.Drain(actionDescriptor.PipeID)
		if err != nil {
			logger.Error("Error obtaining output reader", slog.Any("error", err))
		} else {
			outputForDB, err = io.ReadAll(outputReader)
			if err != nil {
				logger.Error("Error reading the action output", slog.Any("error", err))
			}
		}
		err = r.actionsDB.CloseRecord(actionDescriptor.PipeID, actionErr, outputForDB)
		if err != nil {
			pipeLogger.Error("Error closing action's db record", slog.Any("error", err))
			return
		}
	}
}
