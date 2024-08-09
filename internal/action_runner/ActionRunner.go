package action_runner

import (
	"context"
	"log/slog"
	"sync"
)

type ActionArgs struct {
	Logger *slog.Logger
	Action ActionDescriptor
}

func Listen(ctx context.Context, ch chan ActionArgs, wg *sync.WaitGroup, outputDir string) {
	for {
		select {
		case args := <-ch:
			wg.Add(1)
			go func() {
				defer wg.Done()
				ExecuteAction(ctx, args.Logger, args.Action, outputDir)
			}()
		case <-ctx.Done():
			return
		}
	}
}
