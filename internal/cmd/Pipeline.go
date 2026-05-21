package cmd

import (
	"fmt"
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

	displayPipeDetails(pipe)
	fmt.Print("\n")
}

func displayPipeDetails(pipe actionsdb.PipeLineRecord) {
	var endedAt string
	if pipe.EndedAt.Valid {
		endedAt = pipe.EndedAt.Time.Format(time.DateTime)
	} else {
		endedAt = ""
	}

	fmt.Printf("pipeId     : %s\n", pipe.PipeID)
	fmt.Printf("project    : %s\n", pipe.Project)
	fmt.Printf("deliveryId : %s\n", pipe.DeliveryID)
	fmt.Printf("config     : %s\n", pipe.Config)
	fmt.Printf("error      : %s\n", pipe.Error.String)
	fmt.Printf("created at : %s\n", pipe.CreatedAt.Format(time.DateTime))
	fmt.Printf("ended at   : %s\n", endedAt)
}
