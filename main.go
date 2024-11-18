package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/alecthomas/kong"
	"github.com/religiosa1/git-webhook-receiver/internal/cmd"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

var CLI struct {
	ConfigPath    string                `short:"c" help:"Configuration file name"`
	Version       bool                  `short:"v" help:"Show version information and exit"`
	Serve         struct{}              `cmd:"" default:"1" help:"Run the webhook receiver server (default mode)"`
	Pipeline      cmd.PipelineArgs      `cmd:"" aliases:"pl,get" help:"Display pipeline output"`
	ListPipelines cmd.ListPipelinesArgs `cmd:"" aliases:"ls" help:"Display a list of last N pipelines"`
	Logs          cmd.LogsArgs          `cmd:"" help:"Display logs"`
}

func main() {
	args := kong.Parse(&CLI)

	if CLI.Version {
		showVersion()
		return
	}

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
	case "logs":
		cmd.Logs(cfg, CLI.Logs)
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

func showVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("Build information not available")
		return
	}

	version := info.Main.Version
	if version == "(devel)" {
		var commit string
		var dirty bool
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				commit = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty = setting.Value == "true"
			}
		}
		version += " " + commit
		if dirty {
			version += " dirty"
		}
	}

	fmt.Printf("%s\n", version)
}
