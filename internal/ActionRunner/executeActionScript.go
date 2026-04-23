package ActionRunner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func executeActionScript(ctx context.Context, action config.Action, sysProcAttr *syscall.SysProcAttr, output io.Writer) error {
	script, err := syntax.NewParser().Parse(strings.NewReader(action.Script), "")
	if err != nil {
		return fmt.Errorf("error parsing actions's script: %w", err)
	}

	runner, _ := interp.New(
		interp.ExecHandlers(execHandler(sysProcAttr)),
		interp.StdIO(nil, output, output),
		interp.Dir(action.Cwd),
	)
	return runner.Run(ctx, script)
}

func execHandler(sysProcAttr *syscall.SysProcAttr) func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			hc := interp.HandlerCtx(ctx)
			path, err := interp.LookPathDir(hc.Dir, hc.Env, args[0])
			if err != nil {
				_, _ = fmt.Fprintln(hc.Stderr, err)
				return interp.ExitStatus(127)
			}
			cmd := newCmd(ctx, path, args[1:], sysProcAttr)
			// cmd.Args defaults to [path, args[1:]...]; override to show the shell-visible
			// command name (args[0]) instead of the resolved path in process listings.
			cmd.Args = args
			cmd.Dir = hc.Dir
			cmd.Stdin = hc.Stdin
			cmd.Stdout = hc.Stdout
			cmd.Stderr = hc.Stderr

			err = cmd.Run()

			switch err := err.(type) {
			case *exec.ExitError:
				if status, ok := err.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						if ctx.Err() != nil {
							return ctx.Err()
						}
						return interp.ExitStatus(128 + status.Signal())
					}
					return interp.ExitStatus(status.ExitStatus())
				}
				return interp.ExitStatus(1)
			case *exec.Error:
				_, _ = fmt.Fprintf(hc.Stderr, "%v\n", err)
				return interp.ExitStatus(127)
			default:
				return err
			}
		}
	}
}
