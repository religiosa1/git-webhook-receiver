package actionrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
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

// ErrPipeline marks a failure of the runner's own machinery
// (tmp file, process attrs, environment) as opposed to a failure
// of the user's action itself.
//
// The sentinel is only inspectable via errors.Is while the error is a live
// chain in this process. Once actionsdb persists it, the error is flattened to
// its message string and reloaded as a plain errors.New, so errors.Is against a
// record read back from the DB will not match. Downstream consumers that read
// stored records treat the message as opaque text.
var ErrPipeline = errors.New("pipeline error")

func pipelineError(err error) error {
	return fmt.Errorf("%w: %w", ErrPipeline, err)
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

	// actionErr is a potential error of action run
	var actionErr error

	// Creating tmpOutputMgr first, so we close it last in defer, error is captured in actionErr
	rawOutput, err := r.tmpOutputMgr.Create(actionDesc.PipeID)
	if err != nil {
		logger.Error("Error creating temporary file to capture action's output", slog.Any("error", err))
		actionErr = pipelineError(fmt.Errorf("error creating a temporary file to capture action output: %w", err))
	} else {
		defer func() {
			err := r.tmpOutputMgr.Close(actionDesc.PipeID)
			if err != nil {
				logger.Error("Error closing action output", slog.Any("error", err))
			}
		}()
	}
	if r.actionsDB != nil {
		err := r.actionsDB.CreateRecord(actionDesc.PipeID, actionDesc.Project, args.DeliveryID, args.Hash, actionDesc.Config)
		if err != nil {
			logger.Error("Error creating pipeline record in the db", slog.Any("error", errors.Join(err, actionErr)))
			return
		}
		defer func() {
			var outputForDB []byte
			// rawOutput == nil means we failed to create a tmp file in the first place
			if rawOutput != nil {
				outputReader, err := r.tmpOutputMgr.Drain(actionDesc.PipeID)
				if err != nil {
					logger.Error("Error obtaining output reader", slog.Any("error", err))
				} else {
					outputForDB, err = io.ReadAll(outputReader)
					if err != nil {
						logger.Error("Error reading the action output", slog.Any("error", err))
					}
				}
			}
			err = r.actionsDB.CloseRecord(actionDesc.PipeID, actionErr, outputForDB)
			if err != nil {
				logger.Error("Error closing action's db record", slog.Any("error", err))
				return
			}
		}()
	}
	// Checking tmpOutputMgr was actually created before proceeding
	if rawOutput == nil {
		return
	}

	sysProcAttr, err := getSysProcAttr(actionDesc.Config.User)
	if err != nil {
		logger.Error("Error creating process attributes for action", slog.Any("error", err))
		actionErr = pipelineError(fmt.Errorf("error creating process attributes for action: %w", err))
		return
	}

	if actionDesc.Config.User != "" {
		logger.Debug("Running from a user", slog.String("user", actionDesc.Config.User))
	}

	actionCtx, cancelAction := context.WithTimeout(ctx, actionDesc.Config.Timeout)
	defer cancelAction()

	// tmpDir is the runner-managed temporary directory exposed to the action as
	// $TMPDIR. We create and remove it here (rather than let the action mktemp
	// its own) so cleanup is guaranteed even when the action times out or is
	// killed -- an in-script `trap ... EXIT` does not fire on context cancel.
	var tmpDir string
	if actionDesc.Config.WithTempDir {
		tmpDir, err = os.MkdirTemp("", "git-webhook-receiver-*")
		if err != nil {
			logger.Error("Error creating temporary directory for action", slog.Any("error", err))
			actionErr = pipelineError(fmt.Errorf("error creating temporary directory for action: %w", err))
			return
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				logger.Error("Error removing action's temporary directory", slog.String("dir", tmpDir), slog.Any("error", err))
			}
		}()
		// MkdirTemp makes a 0700 dir owned by the receiver's user. When the
		// action runs as another user, hand ownership over so it can write --
		// the 0700 mode still keeps the (possibly credential-bearing) contents
		// private to that single user.
		if err := chownActionDir(tmpDir, sysProcAttr); err != nil {
			logger.Error("Error setting ownership of action's temporary directory", slog.Any("error", err))
			actionErr = pipelineError(fmt.Errorf("error setting ownership of action's temporary directory: %w", err))
			return
		}
	}

	outputWriter := &overflowWriter{Writer: rawOutput}
	env, err := createEnv(args, tmpDir)
	if err != nil {
		logger.Error("Error building the action environment", slog.Any("error", err))
		actionErr = pipelineError(fmt.Errorf("error building action environment: %w", err))
		return
	}
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
}
