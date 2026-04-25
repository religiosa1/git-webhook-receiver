package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

type ListPipelinesArgs struct {
	File       string `short:"i" help:"Actions db file (default to the file, specified in config)" type:"path"`
	Limit      int    `short:"l" default:"20" help:"Maximum number of pipeline records to output"`
	Skip       int    `short:"s" default:"0" help:"Skip first N entries"`
	Status     string `short:"e" help:"filter by status" enum:"ok,error,pending,any" default:"any"`
	Project    string `short:"p" help:"filter by project"`
	DeliveryID string `short:"d" help:"filter by deliveryId"`
	Format     string `short:"f" help:"output format" enum:"simple,jq,json" default:"simple"`
}

func ListPipelines(cfg config.Config, args ListPipelinesArgs) {
	if args.File == "" {
		args.File = cfg.ActionsDBFile
	}

	dbActions, err := actiondb.New(args.File, cfg.MaxActionsStored, cfg.MaxOutputBytes)
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

	query := actiondb.ListPipelineRecordsQuery{
		Limit:  args.Limit,
		Offset: args.Skip,

		Project:    args.Project,
		DeliveryID: args.DeliveryID,
	}
	query.Status, err = actiondb.ParsePipelineStatus(args.Status)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing pipeline state: %s\n", err)
		// not aborting the execution here, just logging out
	}

	pipeLines, err := dbActions.ListPipelineRecords(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading actions db: %s\n", err)
		os.Exit(ExitCodeActionsDB)
	}

	outputFormatter := getActionRecordOutputFormatter(args.Format)
	err = outputFormatter(pipeLines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting records: %s\n", err)
		os.Exit(ExitCodeOutput)
	}
}

func getActionRecordOutputFormatter(format string) func([]actiondb.PipeLineRecord) error {
	switch format {
	case "simple":
		return formatActionRecordsSimple
	case "jq":
		return formatActionRecordsJq
	case "json":
		return formatActionRecordsJSON
	default:
		panic(fmt.Errorf("unknown formatter type: '%s'", format))
	}
}

func formatActionRecordsSimple(pipelines []actiondb.PipeLineRecord) error {
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
		fmt.Printf("%s-%s %s %s %s %s\n", createAt, endedAt, pl.PipeID, pl.DeliveryID, pl.Project, result)
	}
	return nil
}

func formatActionRecordsJq(pipelines []actiondb.PipeLineRecord) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, pl := range pipelines {
		err := enc.Encode(serialization.PipelineRecord(pl))
		if err != nil {
			return err
		}
	}
	return nil
}

func formatActionRecordsJSON(pipelines []actiondb.PipeLineRecord) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err := enc.Encode(serialization.PipelineRecords(pipelines))
	if err != nil {
		return err
	}
	return nil
}
