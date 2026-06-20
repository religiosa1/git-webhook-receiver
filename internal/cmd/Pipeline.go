package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type PipelineArgs struct {
	PipeID string `arg:"" optional:"" name:"pipeId" help:"Id of the pipeline info to extract (if empty returns the last created pipeline)"`
	File   string `short:"f" help:"Actions db file (default to the file, specified in config)" type:"path"`
}

func Pipeline(cfg config.Config, args PipelineArgs) {
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

	var pipe actionsdb.PipeLineRecord
	if args.PipeID == "" {
		pipe, err = dbActions.GetLastPipelineRecord()
	} else {
		pipe, err = dbActions.GetPipelineRecord(args.PipeID)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get the pipeline record: %s\n", err)
		os.Exit(ExitCodeActionsDB)
	}

	if err = displayPipeDetails(os.Stdout, pipe); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(ExitCodeOutput)
	}
}

func displayPipeDetails(w io.Writer, pipe actionsdb.PipeLineRecord) error {
	var endedAt string
	if pipe.EndedAt != nil {
		endedAt = pipe.EndedAt.Format(time.DateTime)
	}
	print := func(columnName string, value any) error {
		_, err := fmt.Fprintf(w, "%s %s\n", columnName, value)
		return err
	}
	var pipeErr string
	if pipe.Error != nil {
		pipeErr = pipe.Error.Error()
	}
	return errors.Join(
		print("pipeId    ", pipe.PipeID),
		print("project   ", pipe.Project),
		print("deliveryId", pipe.DeliveryID),
		print("config    ", pipe.Config),
		print("error     ", pipeErr),
		print("created at", pipe.CreatedAt.Format(time.DateTime)),
		print("ended at  ", endedAt),
	)
}
