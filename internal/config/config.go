package config

import (
	"fmt"
	"net/url"
	"os/user"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	DefaultMaxActionsStored = 1_000
	DefaultMaxOutputBytes   = 1_048_576 // 1 MiB
)

const (
	DefaultTimeout          = 10 * time.Minute
	DefaultGracefulShutdown = 15 * time.Second
)

type Config struct {
	Addr                    string             `yaml:"addr" env:"ADDR" env-default:"localhost:9090"`
	PublicURL               string             `yaml:"public_url" env:"PUBLIC_URL"`
	DisableAPI              bool               `yaml:"disable_api" env:"DISABLE_API"`
	APIUser                 string             `yaml:"api_user" env:"API_USER" env-default:"admin"`
	APIPassword             Secret             `yaml:"api_password" env:"API_PASSWORD"`
	LogLevel                string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogType                 string             `yaml:"log_type" env:"LOG_TYPE" env-default:"json"`
	LogsDBFile              string             `yaml:"logs_db_file" env:"LOGS_DB_FILE"`
	ActionsDBFile           string             `yaml:"actions_db_file" env:"ACTIONS_DB_FILE" env-default:"actions.sqlite3"`
	MaxActionsStored        int                `yaml:"max_actions_stored" env:"MAX_ACTIONS_STORED" env-default:"1000"` // the same as DefaultMaxActionsStored
	MaxOutputBytes          int                `yaml:"max_output_bytes" env:"MAX_OUTPUT_BYTES" env-default:"1048576"`  // the same as DefaultMaxOutputBytes
	ActionsTimeout          time.Duration      `yaml:"actions_timeout" env:"ACTIONS_TIMEOUT" env-default:"10m"`
	ActionsGracefulShutdown time.Duration      `yaml:"actions_graceful_shutdown" env:"ACTIONS_GRACEFUL_SHUTDOWN" env-default:"15s"`
	Ssl                     SslConfig          `yaml:"ssl" env-prefix:"SSL__"`
	Projects                map[string]Project `yaml:"projects" env-required:"true"`
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
	Authorization Secret   `yaml:"authorization" env:"AUTH" json:"authorization,omitzero"`
	Secret        Secret   `yaml:"secret" env:"SECRET" json:"secret,omitzero"`
	Actions       []Action `yaml:"actions" env-required:"true"`
}

type Action struct {
	On               string        `yaml:"on" env-default:"push" json:"on,omitempty"`
	Branch           string        `yaml:"branch" env-default:"master" json:"branch,omitempty"`
	Cwd              string        `yaml:"cwd" json:"cwd,omitempty"`
	User             string        `yaml:"user" json:"user,omitempty"`
	Script           string        `yaml:"script" json:"script,omitempty"`
	Run              []string      `yaml:"run" json:"run,omitempty"`
	Timeout          time.Duration `yaml:"timeout"`
	GracefulShutdown time.Duration `yaml:"graceful_shutdown"`
}

func Load(configPath string) (Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return cfg, fmt.Errorf("error loading configuration %s: %w", configPath, err)
	}
	applyEnvToProjectAndActions(&cfg)

	if err := validateLogType(cfg.LogType); err != nil {
		return cfg, err
	}
	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return cfg, err
	}

	if cfg.PublicURL != "" {
		if u, err := url.Parse(cfg.PublicURL); err != nil || u.Scheme == "" || u.Host == "" {
			return cfg, fmt.Errorf("public_url must be a valid absolute URL (got %q)", cfg.PublicURL)
		}
	}

	// zeros are overwritten here by clean-env `env-default` so we don't particularly care about them
	if cfg.ActionsTimeout < 0 {
		return cfg, fmt.Errorf("'actions_timeout' must be a non-negative duration")
	} else if cfg.ActionsTimeout == 0 {
		panic("global timeout value should've been set by the env-default in config parsing")
	}
	if cfg.ActionsGracefulShutdown < 0 {
		return cfg, fmt.Errorf("'actions_graceful_shutdown' must be a non-negative duration")
	} else if cfg.ActionsGracefulShutdown == 0 {
		panic("global graceful shutdown value should've been set by the env-default in config parsing")
	}

	projectsWithDefaults, err := validateAndSetDefaultsConfigProjects(cfg.Projects, cfg.ActionsTimeout, cfg.ActionsGracefulShutdown)
	if err != nil {
		return cfg, fmt.Errorf("configs projects validation failed: %w", err)
	}

	cfg.Projects = projectsWithDefaults

	return cfg, nil
}

func validateLogLevel(loglevel string) error {
	switch loglevel {
	case "info", "debug", "warn", "error":
		return nil
	default:
		return fmt.Errorf("incorrect LogLevel value %q. Possible values are 'debug', 'info', 'warn', and 'error'", loglevel)
	}
}

func validateLogType(logType string) error {
	switch logType {
	case "json", "text":
		return nil
	default:
		return fmt.Errorf("unknown log type %q", logType)
	}
}

func validateAndSetDefaultsConfigProjects(
	projects map[string]Project,
	globalTimeout time.Duration,
	globalGracefulShutdown time.Duration,
) (map[string]Project, error) {
	for projectName, project := range projects {
		if err := setDefaultAndCheckRequired(&project); err != nil {
			return nil, fmt.Errorf("project %q has issue with its fields: %w", projectName, err)
		}

		if err := isValidProjectName(projectName); err != nil {
			return nil, fmt.Errorf("bad project name %q: %w", projectName, err)
		}

		if len(project.Actions) == 0 {
			return nil, fmt.Errorf(
				"project %q has no associated actions and can not be executed; "+
					"either add 'actions' list to the project or comment the project out",
				projectName,
			)
		}

		actionsWithDefaults, err := validateAndSetDefaultConfigActions(projectName, project.Actions, globalTimeout, globalGracefulShutdown)
		if err != nil {
			return nil, fmt.Errorf("action validation failed: %w", err)
		}
		project.Actions = actionsWithDefaults
		projects[projectName] = project
	}
	return projects, nil
}

func validateAndSetDefaultConfigActions(projectName string, actions []Action, globalTimeout time.Duration, globalGracefulShutdown time.Duration) ([]Action, error) {
	for i, action := range actions {
		wrapActionErr := func(err error) error {
			return fmt.Errorf(
				"bad action %d (invoked on %q) of project %q: %w",
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
				return nil, wrapActionErr(fmt.Errorf("has a user field = %q, but this user can't be found: %w", action.User, err))
			}
		}

		if action.Timeout == 0 {
			action.Timeout = globalTimeout
		} else if action.Timeout < 0 {
			return nil, wrapActionErr(fmt.Errorf("'timeout' cannot be a negative value"))
		}
		if action.GracefulShutdown == 0 {
			action.GracefulShutdown = globalGracefulShutdown
		} else if action.GracefulShutdown < 0 {
			return nil, wrapActionErr(fmt.Errorf("'graceful_shutdown' cannot be a negative value"))
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

// ParseAddr splits an addr string into network and address.
// Addresses prefixed with "unix://" or "unix:" are treated as Unix socket paths;
// everything else is treated as a TCP address.
func ParseAddr(addr string) (network, address string) {
	if path, found := strings.CutPrefix(addr, "unix://"); found {
		return "unix", path
	}
	if path, found := strings.CutPrefix(addr, "unix:"); found {
		return "unix", path
	}
	return "tcp", addr
}
