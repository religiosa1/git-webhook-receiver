package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/religiosa1/git-webhook-receiver/internal/cmd"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

var CLI struct {
	ConfigPath    string                `short:"c" help:"Configuration file name"`
	Serve         struct{}              `cmd:"" default:"1" help:"Run the webhook receiver server (default mode)"`
	Pipeline      cmd.PipelineArgs      `cmd:"" aliases:"pl,get" help:"Display pipeline output"`
	ListPipelines cmd.ListPipelinesArgs `cmd:"" aliases:"ls" help:"Display a list of last N pipelines"`
}

func main() {
	args := kong.Parse(&CLI)

	cfg, err := config.Load(getEnvConfigPath(CLI.ConfigPath))
	if err != nil {
		fmt.Printf("Unable to load configuration file, aborting: %s\n", err)
		os.Exit(cmd.ExitReadConfig)
	}

	switch args.Command() {
	case "pipeline", "pipeline <pipeId>":
		cmd.Pipeline(cfg, CLI.Pipeline)
	case "list-pipelines":
		cmd.ListPipelines(cfg, CLI.ListPipelines)
	default:
		cmd.Serve(cfg)
	}
}

func getEnvConfigPath(configPath string) string {
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		configPath = "config.yml"
	}
	return configPath
}
