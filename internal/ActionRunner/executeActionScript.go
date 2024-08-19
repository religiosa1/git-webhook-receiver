package ActionRunner

import (
	"context"
	"fmt"
	"io"
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

func executeActionScript(ctx context.Context, action config.Action, sysProcAttr *syscall.SysProcAttr, output io.Writer) error {
	script, err := syntax.NewParser().Parse(strings.NewReader(action.Script), "")
	if err != nil {
		return fmt.Errorf("error parsing actions's script: %w", err)
	}

	runner, _ := interp.New(
		interp.ExecHandler(execHandler(30*time.Second, sysProcAttr)),
		interp.StdIO(nil, output, output),
		interp.Dir(action.Cwd),
	)
	return runner.Run(ctx, script)
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
