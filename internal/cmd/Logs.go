package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
)

type LogsArgs struct {
	File       string   `short:"f" help:"logs db file (default to the file, specified in config)" type:"path"`
	Limit      int      `short:"l" default:"20" help:"Maximum number of log entries to output"`
	Skip       int      `short:"s" default:"0" help:"Skip first N entries"`
	Levels     []string `short:"e" help:"filter by levels" enum:"debug,info,warn,error"`
	Project    string   `short:"p" help:"filter by project"`
	DeliveryId string   `short:"d" help:"filter by deliveryId"`
	PipeId     string   `short:"a" help:"filter by action's pipeId"`
	Message    string   `short:"m" help:"filter by message"`
	Format     string   `short:"F" help:"output format" enum:"simple,jq,json" default:"simple" `
}

func Logs(cfg config.Config, args LogsArgs) {
	if args.File == "" {
		args.File = cfg.LogsDbFile
	}

	outputFormmater := getLogOutputFormatter(args.Format)
	if outputFormmater == nil {
		fmt.Printf("unknown output format")
		os.Exit(1)
	}

	query := logsDb.GetEntryFilteredQuery{
		GetEntryQuery: logsDb.GetEntryQuery{
			PageSize: args.Limit,
		},
		Project:    args.Project,
		DeliveryId: args.DeliveryId,
		PipeId:     args.PipeId,
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
		fmt.Printf("Error opening actions db: %s\n", err)
		os.Exit(ExitCodeLoggerDb)
	}
	defer dbLogs.Close()

	records, err := dbLogs.GetEntryFiltered(query)
	if err != nil {
		fmt.Printf("Error retrieving the records: %s", err)
		os.Exit(ExitCodeLoggerDb)
	}
	outputFormmater(records)
}

type prettyLogEntry struct {
	Level      string `json:"level"`
	Project    string `json:"project,omitempty"`
	DeliveryId string `json:"delivery_id,omitempty"`
	PipeId     string `json:"pipe_id,omitempty"`
	Message    string `json:"message"`
	Data       string `json:"data"`
	Ts         string `json:"ts"`
}

func prettyfyLogEntry(entry logsDb.LogEntry) prettyLogEntry {
	p := prettyLogEntry{
		Project:    entry.Project.String,
		DeliveryId: entry.DeliveryId.String,
		PipeId:     entry.PipeId.String,
		Message:    entry.Message,
		Data:       entry.Data,
		Ts:         time.Unix(entry.Ts, 0).Format(time.DateTime),
	}
	switch slog.Level(entry.Level) {
	case slog.LevelDebug:
		p.Level = "debug"
	case slog.LevelInfo:
		p.Level = "info"
	case slog.LevelWarn:
		p.Level = "warn"
	case slog.LevelError:
		p.Level = "error"
	}
	return p
}

func getLogOutputFormatter(format string) func([]logsDb.LogEntry) {
	switch format {
	case "simple":
		return formatLogRecordsSimple
	case "jq":
		return formatLogRecordsJq
	case "json":
		return formatLogRecordsJson
	default:
		return nil
	}
}

func formatLogRecordsSimple(entries []logsDb.LogEntry) {
	for _, e := range entries {
		ts := time.Unix(e.Ts, 0).Format(time.DateTime)
		fmt.Printf("%s %s %s %s %s %s\n", ts, e.Message, e.Project.String, e.DeliveryId.String, e.PipeId.String, e.Data)
	}
}

func formatLogRecordsJq(entries []logsDb.LogEntry) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, entry := range entries {
		enc.Encode(prettyfyLogEntry(entry))
	}
}

func formatLogRecordsJson(entries []logsDb.LogEntry) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	prettyRecords := make([]prettyLogEntry, len(entries))
	for i := 0; i < len(entries); i++ {
		prettyRecords[i] = prettyfyLogEntry(entries[i])
	}
	enc.Encode(prettyRecords)
}
