package config

import (
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LogLevel string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogFile  string             `yaml:"log_file" env:"LOG_FILE" env-default:"deploy.log"`
	Host     string             `yaml:"host" env:"HOST" env-default:"localhost"`
	Port     int16              `yaml:"port" env:"PORT" env-default:"9090"`
	Projects map[string]Project `yaml:"projects"`
}

type Project struct {
	GitProvider string   `yaml:"git_provider" env-default:"gitea"`
	Cwd         string   `yaml:"cwd"`
	Repo        string   `yaml:"repo" env-required:"true"`
	Actions     []Action `yaml:"actions" env-required:"true"`
}

type Action struct {
	On         string `yaml:"on" env-default:"push"`
	Branch     string `yaml:"branch" env-default:"*"`
	User       string `yaml:"user"`
	Script     string `yaml:"script"`
	ScriptPath string `yaml:"script_path"`
}

func MustLoad(configPath string) *Config {
	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("Error loading configuration %s: %s", configPath, err)
	}

	switch cfg.LogLevel {
	case "":
		cfg.LogLevel = "info"
	case "info", "debug", "warn", "error":
		// everything is ok, no action needed
	default:
		log.Fatalf("Incorrect LogLevel value '%s'. Possible values are 'debug', 'info', 'warn', and 'error", cfg.LogLevel)
	}

	for projectName, project := range cfg.Projects {
		if len(project.Actions) == 0 {
			log.Fatalf(
				"Project '%s' has no associated actions and can not be executed.\n"+
					"Either add 'actions' list to the project or comment the project out.",
				projectName,
			)
		}
		for i, action := range project.Actions {
			if action.Script == "" && action.ScriptPath == "" {
				log.Fatalf(
					"Action %d (invoked on %s) of project '%s' has neither 'script' nor 'script_path' fields "+
						"and can not be executed", i+1, action.On,
					projectName,
				)
			}
		}
	}

	return &cfg
}
