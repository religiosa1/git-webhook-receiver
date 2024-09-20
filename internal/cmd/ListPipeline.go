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
	DeliveryId string `short:"d" help:"filter by deliveryId"`
	Format     string `short:"f" help:"output format" enum:"simple,jq,json" default:"simple"`
}

func ListPipelines(cfg config.Config, args ListPipelinesArgs) {
	if args.File == "" {
		args.File = cfg.ActionsDbFile
	}

	outputFormmater := getActionRecordOutputFormatter(args.Format)

	dbActions, err := actiondb.New(args.File)
	if err != nil {
		fmt.Printf("Error opening actions db: %s\n", err)
		os.Exit(ExitCodeActionsDb)
	}
	defer dbActions.Close()

	query := actiondb.ListPipelineRecordsQuery{
		Limit:  args.Limit,
		Offset: args.Skip,

		Project:    args.Project,
		DeliveryId: args.DeliveryId,
	}
	query.Status, _ = actiondb.ParsePipelineStatus(args.Status)

	pipeLines, err := dbActions.ListPipelineRecords(query)
	if err != nil {
		fmt.Printf("Error reading actions db: %s\n", err)
		os.Exit(ExitCodeActionsDb)
	}

	outputFormmater(pipeLines)
}

func getActionRecordOutputFormatter(format string) func([]actiondb.PipeLineRecord) {
	switch format {
	case "simple":
		return formatActionRecordsSimple
	case "jq":
		return formatActionRecordsJq
	case "json":
		return formatActionRecordsJson
	default:
		return nil
	}
}

func formatActionRecordsSimple(pipelines []actiondb.PipeLineRecord) {
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
		fmt.Printf("%s-%s %s %s %s %s\n", createAt, endedAt, pl.PipeId, pl.DeliveryId, pl.Project, result)
	}
}

func formatActionRecordsJq(pipelines []actiondb.PipeLineRecord) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, pl := range pipelines {
		enc.Encode(serialization.PipelineRecord(pl))
	}
}

func formatActionRecordsJson(pipelines []actiondb.PipeLineRecord) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(serialization.PipelineRecords(pipelines))
}
