package cmd

import (
	"fmt"
	"os"
	"time"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type PipelineArgs struct {
	PipeID     string `arg:"" optional:"" name:"pipeId" help:"Id of the pipeline output to extract (if empty returns the last created pipeline)"`
	File       string `short:"f" help:"Actions db file (default to the file, specified in config)" type:"path"`
	Info       bool   `short:"i" help:"Display only pipeline general info, without its output"`
	OutputOnly bool   `short:"o" help:"Display only pipeline output, without general info"`
}

func Pipeline(cfg config.Config, args PipelineArgs) {
	if args.Info && args.OutputOnly {
		fmt.Fprintln(os.Stderr, "Unable to specify both header-only and output-only flags")
		os.Exit(ExitCodeCLI)
	}
	if args.File == "" {
		args.File = cfg.ActionsDBFile
	}
	dbActions, err := actiondb.New(args.File, cfg.MaxActionsStored)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening actions db: %s\n", err)
		os.Exit(ExitCodeActionsDB)
	}
	defer func() {
		err := dbActions.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error closing actions DB: %s\n", err)
			os.Exit(ExitCodeActionsDB)
		}
	}()

	pipe, err := dbActions.GetPipelineRecord(args.PipeID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get the pipeline record: %s\n", err)
		os.Exit(ExitCodeActionsDB)
	}

	if !args.OutputOnly {
		displayPipeDetails(pipe)
		if !args.Info {
			fmt.Print("\n")
		}
	}
	if !args.Info && pipe.Output.Valid {
		fmt.Print(pipe.Output.String)
	}
}

func displayPipeDetails(pipe actiondb.PipeLineRecord) {
	var endedAt string
	if pipe.EndedAt.Valid {
		endedAt = time.Unix(pipe.EndedAt.Int64, 0).Format(time.DateTime)
	} else {
		endedAt = ""
	}

	fmt.Printf("pipeId        : %s\n", pipe.PipeID)
	fmt.Printf("project       : %s\n", pipe.Project)
	fmt.Printf("deliveryId    : %s\n", pipe.DeliveryID)
	fmt.Printf("config        : %s\n", pipe.Config)
	fmt.Printf("error         : %s\n", pipe.Error.String)
	fmt.Printf("output length : %s\n", formatLength(pipe.Output))
	fmt.Printf("created at    : %s\n", time.Unix(pipe.CreatedAt, 0).Format(time.DateTime))
	fmt.Printf("ended at      : %s\n", endedAt)
}
