package actionrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
)

// ActionArgs are arguments passed to run an action/pipeline by an ActionRunner
type ActionArgs struct {
	Logger     *slog.Logger
	ActionDesc ActionDescriptor
	DeliveryID string
	Hash       string
	Event      string
	Branch     string
}

type ActionRunner struct {
	wg           *sync.WaitGroup
	actionsDB    *actionsdb.ActionDB
	tmpOutputMgr tmpoutput.Manager
	listenDone   chan struct{}
	semaphore    chan struct{}
}

// overflowWriter wraps an io.Writer and records whether ErrOutputTooLarge was ever returned.
// This is needed because ErrOutputTooLarge gets lost in transit -- cmd can be killed
// with whatever other status,  we want to capture the actual cause at the source.
type overflowWriter struct {
	io.Writer
	overflowed bool
}

func (w *overflowWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if errors.Is(err, tmpoutput.ErrOutputTooLarge) {
		w.overflowed = true
	}
	return n, err
}

func New(
	ctx context.Context,
	actionArgsStream <-chan ActionArgs,
	maxConcurrentActions int,
	actionsDB *actionsdb.ActionDB,
	tmpOutputMgr tmpoutput.Manager,
) *ActionRunner {
	r := ActionRunner{
		wg:           &sync.WaitGroup{},
		actionsDB:    actionsDB,
		tmpOutputMgr: tmpOutputMgr,
		listenDone:   make(chan struct{}),
		semaphore:    make(chan struct{}, maxConcurrentActions),
	}
	go r.listen(ctx, actionArgsStream)
	return &r
}

// Wait waits for the channel to close and all of actions to finish
func (r *ActionRunner) Wait() {
	<-r.listenDone // to make sure all wg.Go in listen are issued
	r.wg.Wait()
}

func (r *ActionRunner) listen(ctx context.Context, actionArgsStream <-chan ActionArgs) {
	defer func() {
		close(r.listenDone)
		close(r.semaphore)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case args, ok := <-actionArgsStream:
			if !ok {
				return
			}
			r.semaphore <- struct{}{}
			r.wg.Go(func() {
				defer func() {
					<-r.semaphore
				}()
				r.executeAction(ctx, args)
			})
		}
	}
}

//------------------------------------------------------------------------------
// Private parts

func (r *ActionRunner) executeAction(
	ctx context.Context,
	args ActionArgs,
) {
	actionDesc := args.ActionDesc
	logger := args.Logger
	logger.Info("Running action", slog.Int("action_index", actionDesc.Index))

	if r.actionsDB != nil {
		err := r.actionsDB.CreateRecord(actionDesc.PipeID, actionDesc.Project, args.DeliveryID, args.Hash, actionDesc.Config)
		if err != nil {
			logger.Error("Error creating pipeline record in the db", slog.Any("error", err))
			return
		}
	}

	rawOutput, err := r.tmpOutputMgr.Create(actionDesc.PipeID)
	if err != nil {
		logger.Error("Error creating temporary file to capture action's output", slog.Any("error", err))
		return
	}
	outputWriter := &overflowWriter{Writer: rawOutput}
	defer func() {
		err := r.tmpOutputMgr.Close(actionDesc.PipeID)
		if err != nil {
			logger.Error("Error closing action output", slog.Any("error", err))
		}
	}()

	sysProcAttr, err := getSysProcAttr(actionDesc.Config.User)
	if err != nil {
		logger.Error("Error creating process attributes for action", slog.Any("error", err))
		return
	}

	if actionDesc.Config.User != "" {
		logger.Debug("Running from a user", slog.String("user", actionDesc.Config.User))
	}

	actionCtx, cancelAction := context.WithTimeout(ctx, actionDesc.Config.Timeout)
	defer cancelAction()

	env, err := createEnv(args)
	if err != nil {
		logger.Error("Error building the action environment", slog.Any("error", err))
		return
	}
	var actionErr error
	if len(actionDesc.Config.Run) > 0 {
		logger.Debug("Running the command", slog.Any("command", actionDesc.Config.Run))
		actionErr = executeActionRun(actionCtx, actionDesc.Config, env, sysProcAttr, outputWriter)
	} else {
		logger.Debug("Running the script", slog.String("script", actionDesc.Config.Script))
		actionErr = executeActionScript(actionCtx, actionDesc.Config, env, sysProcAttr, outputWriter)
	}

	if outputWriter.overflowed {
		actionErr = fmt.Errorf("action output exceeded the maximum allowed size: %w", tmpoutput.ErrOutputTooLarge)
	}

	if actionErr != nil {
		logger.Error("Error while running the action", slog.Any("error", actionErr))
	} else {
		logger.Info("Action successfully finished")
	}

	if r.actionsDB != nil {
		var outputForDB []byte
		outputReader, err := r.tmpOutputMgr.Drain(actionDesc.PipeID)
		if err != nil {
			logger.Error("Error obtaining output reader", slog.Any("error", err))
		} else {
			outputForDB, err = io.ReadAll(outputReader)
			if err != nil {
				logger.Error("Error reading the action output", slog.Any("error", err))
			}
		}
		err = r.actionsDB.CloseRecord(actionDesc.PipeID, actionErr, outputForDB)
		if err != nil {
			logger.Error("Error closing action's db record", slog.Any("error", err))
			return
		}
	}
}
