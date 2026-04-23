package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

type LogsArgs struct {
	File       string   `short:"i" help:"logs db file (default to the file, specified in config)" type:"path"`
	Limit      int      `short:"l" default:"20" help:"Maximum number of log entries to output"`
	Skip       int      `short:"s" default:"0" help:"Skip first N entries"`
	Levels     []string `short:"e" help:"filter by levels" enum:"debug,info,warn,error"`
	Project    string   `short:"p" help:"filter by project"`
	DeliveryID string   `short:"d" help:"filter by deliveryId"`
	PipeID     string   `short:"a" help:"filter by action's pipeId"`
	Message    string   `short:"m" help:"filter by message"`
	Format     string   `short:"f" help:"output format" enum:"simple,jq,json" default:"simple" `
}

func Logs(cfg config.Config, args LogsArgs) {
	if args.File == "" {
		args.File = cfg.LogsDBFile
	}

	outputFormatter := getLogOutputFormatter(args.Format)
	if outputFormatter == nil {
		fmt.Printf("unknown output format")
		os.Exit(1)
	}

	query := logsDb.GetEntryFilteredQuery{
		GetEntryQuery: logsDb.GetEntryQuery{
			PageSize: args.Limit,
		},
		Project:    args.Project,
		DeliveryID: args.DeliveryID,
		PipeID:     args.PipeID,
		Message:    args.Message,
		Offset:     args.Skip,
	}

	query.Levels = make([]int, 0)
	for _, lvl := range args.Levels {
		l, err := logsDb.ParseLogLevel(lvl)
		if err == nil {
			query.Levels = append(query.Levels, l)
		}
	}

	dbLogs, err := logsDb.New(args.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening logs db: %s\n", err)
		os.Exit(ExitCodeLoggerDB)
	}
	defer func() {
		err := dbLogs.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logs db: %s\n", err)
			os.Exit(ExitCodeLoggerDB)
		}
	}()

	records, err := dbLogs.GetEntryFiltered(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving the logs records: %s", err)
		os.Exit(ExitCodeLoggerDB)
	}
	err = outputFormatter(records)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting records: %s\n", err)
		os.Exit(ExitCodeOutput)
	}
}

func getLogOutputFormatter(format string) func([]logsDb.LogEntry) error {
	switch format {
	case "simple":
		return formatLogRecordsSimple
	case "jq":
		return formatLogRecordsJq
	case "json":
		return formatLogRecordsJSON
	default:
		panic(fmt.Errorf("unknown formatter type: '%s'", format))
	}
}

func formatLogRecordsSimple(entries []logsDb.LogEntry) error {
	for _, e := range entries {
		ts := time.Unix(e.TS, 0).Format(time.DateTime)
		fmt.Printf("%s %s %s %s %s %s\n", ts, e.Message, e.Project.String, e.DeliveryID.String, e.PipeID.String, e.Data)
	}
	return nil
}

func formatLogRecordsJq(entries []logsDb.LogEntry) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, entry := range entries {
		err := enc.Encode(serialization.LogEntry(entry))
		if err != nil {
			return err
		}
	}
	return nil
}

func formatLogRecordsJSON(entries []logsDb.LogEntry) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err := enc.Encode(serialization.LogEntries((entries)))
	return err
}
