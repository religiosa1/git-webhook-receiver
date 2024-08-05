package action_runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/religiosa1/webhook-receiver/internal/config"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func executeActionScript(logger *slog.Logger, action config.Action, streams actionIoStreams) {
	logger.Debug("Running script", slog.String("script", action.Script))
	script, err := syntax.NewParser().Parse(strings.NewReader(action.Script), "")
	if err != nil {
		logger.Error("Error parsing action's script", slog.Any("error", err))
		return
	}

	var sysProcAttr *syscall.SysProcAttr
	if runtime.GOOS != "windows" && action.User != "" {
		if spa, err := applyUser(action.User); err != nil {
			logger.Error(
				"Unable to run action from the specified user:",
				slog.String("username", action.User),
				slog.Any("error", err),
			)
		} else {
			logger.Debug("Running the command from a user", slog.String("user", action.User))
			sysProcAttr = spa
		}
	}

	runner, _ := interp.New(
		interp.ExecHandler(execHandler(30*time.Second, sysProcAttr)),
		interp.StdIO(nil, streams.Stdout, streams.Stderr),
		interp.Dir(action.Cwd),
	)
	if err := runner.Run(context.TODO(), script); err != nil {
		logger.Error("Script execution ended with an error", slog.Any("error", err))
	} else {
		logger.Info("Script successfully finished")
	}
}

// TODO investigate the possibility of integrating it in interp itself (fork or PRS)
func execHandler(killTimeout time.Duration, sysProcAttr *syscall.SysProcAttr) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		path, err := interp.LookPathDir(hc.Dir, hc.Env, args[0])
		if err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.NewExitStatus(127)
		}
		cmd := exec.Cmd{
			Path: path,
			Args: args,
			// Env:         execEnv(hc.Env),
			Dir:         hc.Dir,
			Stdin:       hc.Stdin,
			Stdout:      hc.Stdout,
			Stderr:      hc.Stderr,
			SysProcAttr: sysProcAttr,
		}

		err = cmd.Start()
		if err == nil {
			if done := ctx.Done(); done != nil {
				go func() {
					<-done

					if killTimeout <= 0 || runtime.GOOS == "windows" {
						_ = cmd.Process.Signal(os.Kill)
						return
					}

					// TODO: don't temporarily leak this goroutine
					// if the program stops itself with the
					// interrupt.
					go func() {
						time.Sleep(killTimeout)
						_ = cmd.Process.Signal(os.Kill)
					}()
					_ = cmd.Process.Signal(os.Interrupt)
				}()
			}

			err = cmd.Wait()
		}

		switch err := err.(type) {
		case *exec.ExitError:
			// started, but errored - default to 1 if OS
			// doesn't have exit statuses
			if status, ok := err.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return interp.NewExitStatus(uint8(128 + status.Signal()))
				}
				return interp.NewExitStatus(uint8(status.ExitStatus()))
			}
			return interp.NewExitStatus(1)
		case *exec.Error:
			// did not start
			fmt.Fprintf(hc.Stderr, "%v\n", err)
			return interp.NewExitStatus(127)
		default:
			return err
		}
	}
}
