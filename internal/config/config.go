package config

import (
	"fmt"
	"os/user"
	"runtime"
	"unicode"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	DefaultMaxActionsStored   = 1_000
	DefaultTimeoutSeconds     = 600
	DefaultGracefulShutdownMS = 15_000
)

type Config struct {
	Addr               string             `yaml:"addr" env:"ADDR" env-default:"localhost:9090"`
	PublicURL          string             `yaml:"public_url" env:"PUBLIC_URL"`
	DisableAPI         bool               `yaml:"disable_api" env:"DISABLE_API"`
	APIUser            string             `yaml:"api_user" env:"API_USER" env-default:"admin"`
	APIPassword        string             `yaml:"api_password" env:"API_PASSWORD"`
	LogLevel           string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogsDBFile         string             `yaml:"logs_db_file" env:"LOGS_DB_FILE"`
	ActionsDBFile      string             `yaml:"actions_db_file" env:"ACTIONS_DB_FILE" env-default:"actions.sqlite3"`
	MaxActionsStored   int                `yaml:"max_actions_stored" env:"MAX_ACTIONS_STORED" env-default:"1000"` // the same as DefaultMaxActionsStored
	TimeoutSeconds     int                `yaml:"timeout_seconds" env:"TIMEOUT_SECONDS" env-default:"600"`
	GracefulShutdownMS int                `yaml:"graceful_shutdown_ms" env:"GRACEFUL_SHUTDOWN_MS" env-default:"15000"`
	Ssl                SslConfig          `yaml:"ssl" env-prefix:"SSL__"`
	Projects           map[string]Project `yaml:"projects" env-required:"true"`
}

type SslConfig struct {
	CertFilePath string `yaml:"cert_file_path" env:"CERT_FILE_PATH"`
	KeyFilePath  string `yaml:"key_file_path" env:"KEY_FILE_PATH"`
}

// For both Projects and Actions only the fields marked with env:"..." struct
// tag can be set through the env variables. See [applyEnvToProjectAndActions]

type Project struct {
	GitProvider   string   `yaml:"git_provider" env-default:"github"`
	Repo          string   `yaml:"repo" env-required:"true"`
	Authorization string   `yaml:"authorization" env:"AUTH"`
	Secret        string   `yaml:"secret" env:"SECRET"`
	Actions       []Action `yaml:"actions" env-required:"true"`
}

type Action struct {
	On                 string   `yaml:"on" env-default:"push" json:"on,omitempty"`
	Branch             string   `yaml:"branch" env-default:"master" json:"branch,omitempty"`
	Cwd                string   `yaml:"cwd" json:"cwd,omitempty"`
	User               string   `yaml:"user" json:"user,omitempty"`
	Script             string   `yaml:"script" json:"script,omitempty"`
	Run                []string `yaml:"run" json:"run,omitempty"`
	TimeoutSeconds     int      `yaml:"timeout_seconds"`
	GracefulShutdownMS int      `yaml:"graceful_shutdown_ms"`
}

func Load(configPath string) (Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return cfg, fmt.Errorf("error loading configuration %s: %w", configPath, err)
	}
	applyEnvToProjectAndActions(&cfg)

	switch cfg.LogLevel {
	case "":
		cfg.LogLevel = "info"
	case "info", "debug", "warn", "error":
		// everything is ok, no action needed
	default:
		return cfg, fmt.Errorf("incorrect LogLevel value '%s'. Possible values are 'debug', 'info', 'warn', and 'error", cfg.LogLevel)
	}

	// zeros are overwritten here by clean-env `env-default` so we don't particularly care about them
	if cfg.TimeoutSeconds < 0 {
		return cfg, fmt.Errorf("'timeout_seconds' must be a non-negative integer")
	} else if cfg.TimeoutSeconds == 0 {
		panic("global timeout value should've been set by the env-default in config parsing")
	}
	if cfg.GracefulShutdownMS < 0 {
		return cfg, fmt.Errorf("'graceful_shutdown_ms' must be a non-negative integer")
	} else if cfg.GracefulShutdownMS == 0 {
		panic("global gracefulShutdownMs value should've been set by the env-default in config parsing")
	}

	projectsWithDefaults, err := validateAndSetDefaultsConfigProjects(cfg.Projects, cfg.TimeoutSeconds, cfg.GracefulShutdownMS)
	if err != nil {
		return cfg, fmt.Errorf("configs projects validation failed: %w", err)
	}

	cfg.Projects = projectsWithDefaults

	return cfg, nil
}

func validateAndSetDefaultsConfigProjects(
	projects map[string]Project,
	globalTimeoutSeconds int,
	globalGracefulShutdownMS int,
) (map[string]Project, error) {
	for projectName, project := range projects {
		if err := setDefaultAndCheckRequired(&project); err != nil {
			return nil, fmt.Errorf("project '%s' has issue with its fields: %w", projectName, err)
		}

		if err := isValidProjectName(projectName); err != nil {
			return nil, fmt.Errorf("bad project name '%s': %w", projectName, err)
		}

		if len(project.Actions) == 0 {
			return nil, fmt.Errorf(
				"project '%s' has no associated actions and can not be executed; "+
					"either add 'actions' list to the project or comment the project out",
				projectName,
			)
		}

		actionsWithDefaults, err := validateAndSetDefaultConfigActions(projectName, project.Actions, globalTimeoutSeconds, globalGracefulShutdownMS)
		if err != nil {
			return nil, fmt.Errorf("action validation failed: %w", err)
		}
		project.Actions = actionsWithDefaults
		projects[projectName] = project
	}
	return projects, nil
}

func validateAndSetDefaultConfigActions(projectName string, actions []Action, globalTimeoutSeconds int, globalGracefulShutdownMS int) ([]Action, error) {
	for i, action := range actions {
		wrapActionErr := func(err error) error {
			return fmt.Errorf(
				"bad action %d (invoked on %s) of project '%s': %w",
				i+1,
				action.On,
				projectName,
				err,
			)
		}

		if err := setDefaultAndCheckRequired(&action); err != nil {
			return nil, wrapActionErr(fmt.Errorf("action  has issue with its fields: %w", err))
		}
		if action.Script == "" && len(action.Run) == 0 {
			return nil, wrapActionErr(fmt.Errorf("has neither 'script' nor 'run' fields and can not be executed"))
		}
		if action.Script != "" && len(action.Run) > 0 {
			return nil, wrapActionErr(fmt.Errorf("has both 'script' and 'run' simultaneously, you must use one"))
		}

		if runtime.GOOS != "windows" && action.User != "" {
			if _, err := user.Lookup(action.User); err != nil {
				return nil, wrapActionErr(fmt.Errorf("has a user field = '%s', but this user can't be found: %w", action.User, err))
			}
		}

		if action.TimeoutSeconds == 0 {
			action.TimeoutSeconds = globalTimeoutSeconds
		} else if action.TimeoutSeconds < 0 {
			return nil, wrapActionErr(fmt.Errorf("'timeout_seconds' cannot be a negative value"))
		}
		if action.GracefulShutdownMS == 0 {
			action.GracefulShutdownMS = globalGracefulShutdownMS
		} else if action.GracefulShutdownMS < 0 {
			return nil, wrapActionErr(fmt.Errorf("'graceful_shutdown_ms' cannot be a negative value"))
		}

		actions[i] = action
	}
	return actions, nil
}

func isValidProjectName(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("project name can't be empty")
	}

	if s[0] == '_' {
		return fmt.Errorf("name can't start with '_' symbol")
	}

	var lastRune rune
	for _, r := range s {
		if r == '_' && lastRune == '_' {
			return fmt.Errorf("name can't contain two or more consecutive '_' chars")
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return fmt.Errorf("name can only contain chars from range [a-Z0-9_-]")
		}
		lastRune = r
	}

	return nil
}

const maskValue = "********"

func (cfg Config) MaskSensitiveData() Config {
	maskedCfg := cfg

	if maskedCfg.APIPassword != "" {
		maskedCfg.APIPassword = maskValue
	}

	maskedCfg.Projects = make(map[string]Project, 0)

	for projectName, project := range cfg.Projects {
		maskedCfg.Projects[projectName] = project.MaskSensitiveData()
	}

	return maskedCfg
}

func (project Project) MaskSensitiveData() Project {
	maskedProject := project

	if maskedProject.Secret != "" {
		maskedProject.Secret = maskValue
	}
	if maskedProject.Authorization != "" {
		maskedProject.Authorization = maskValue
	}

	return maskedProject
}
