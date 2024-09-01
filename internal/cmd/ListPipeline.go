package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type ListPipelinesArgs struct {
	File   string `help:"Actions db file (default to the file, specified in config)" type:"path"`
	Limit  int    `short:"l" default:"20" help:"Maximum number of pipeline record to output"`
	Format string `default:"short" help:"Record display type"`
}

func ListPipelines(cfg config.Config, args ListPipelinesArgs) {
	if args.File == "" {
		args.File = cfg.ActionsDbFile
	}
	actionDb, err := actiondb.New(args.File)
	if err != nil {
		log.Printf("Error opening actions db: %s\n", err)
		os.Exit(ExitCodeActionsDb)
	}

	pipeLines, err := actionDb.ListPipelineRecords(args.Limit)
	if err != nil {
		log.Printf("Error reading actions db: %s\n", err)
		os.Exit(ExitCodeActionsDb)
	}

	switch args.Format {
	case "short":
		FormatShort(pipeLines)
	default:
		log.Printf("Unkown display format '%s'\n", args.Format)
		os.Exit(1)
	}
}

func FormatShort(pipelines []actiondb.PipeLineRecord) {
	for _, pl := range pipelines {
		createAt := time.Unix(pl.CreatedAt, 0).Format(time.DateTime)

		var endedAt string
		if pl.EndedAt.Valid {
			endedAt = time.Unix(pl.EndedAt.Int64, 0).Format(time.TimeOnly)
		} else {
			endedAt = "..."
		}

		var result string
		if pl.Error.Valid {
			result = pl.Error.String
		} else {
			result = "ok"
		}
		fmt.Printf("%s-%s %s %s\n", createAt, endedAt, pl.PipeId, result)
	}
}
