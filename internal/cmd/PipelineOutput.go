package cmd

import (
	"fmt"
	"os"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type PipelineOutputArgs struct {
	PipeID string `arg:"" optional:"" name:"pipeId" help:"Id of the pipeline output to extract (if empty returns the last created pipeline)"`
	File   string `short:"f" help:"Actions db file (default to the file, specified in config)" type:"path"`
}

func PipelineOutput(cfg config.Config, args PipelineOutputArgs) {
	if args.File == "" {
		args.File = cfg.ActionsDBFile
	}
	dbActions, err := actionsdb.New(args.File, cfg.MaxActionsStored)
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

	var output []byte
	if args.PipeID == "" {
		output, err = dbActions.GetLastPipelineOutput()
	} else {
		output, err = dbActions.GetPipelineOutput(args.PipeID)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get the pipeline output: %s\n", err)
		os.Exit(ExitCodeActionsDB)
	}

	_, err = os.Stdout.Write(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing the output: %s\n", err)
		os.Exit(ExitCodeRun)
	}
}
