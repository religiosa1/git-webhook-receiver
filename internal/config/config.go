package config

import (
	"fmt"
	"net/url"
	"os/user"
	"runtime"
	"slices"
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
	DisableUI               bool               `yaml:"disable_ui" env:"DISABLE_UI"`
	DisableAPI              bool               `yaml:"disable_api" env:"DISABLE_API"`
	AuthUser                string             `yaml:"auth_user" env:"AUTH_USER" env-default:"admin"`
	AuthPassword            Secret             `yaml:"auth_password" env:"AUTH_PASSWORD"`
	AuthRealm               string             `yaml:"auth_realm" env:"AUTH_REALM" env-default:"Git Webhook Receiver"`
	LogLevel                string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogType                 string             `yaml:"log_type" env:"LOG_TYPE" env-default:"json"`
	LogsDBFile              string             `yaml:"logs_db_file" env:"LOGS_DB_FILE" env-default:"logs.sqlite3"`
	ActionsDBFile           string             `yaml:"actions_db_file" env:"ACTIONS_DB_FILE" env-default:"actions.sqlite3"`
	MaxActionsStored        int                `yaml:"max_actions_stored" env:"MAX_ACTIONS_STORED" env-default:"1000"` // the same as DefaultMaxActionsStored
	MaxOutputBytes          int                `yaml:"max_output_bytes" env:"MAX_OUTPUT_BYTES" env-default:"1048576"`  // the same as DefaultMaxOutputBytes
	MaxConcurrentActions    int                `yaml:"max_concurrent_actions" env:"MAX_CONCURRENT_ACTIONS" env-default:"8"`
	ActionsTimeout          time.Duration      `yaml:"actions_timeout" env:"ACTIONS_TIMEOUT" env-default:"10m"`
	ActionsGracefulShutdown time.Duration      `yaml:"actions_graceful_shutdown" env:"ACTIONS_GRACEFUL_SHUTDOWN" env-default:"15s"`
	Ssl                     SslConfig          `yaml:"ssl" env-prefix:"SSL__"`
	Environment             EnvList            `yaml:"environment"`
	User                    string             `yaml:"user"`
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
	Environment   EnvList  `yaml:"environment" json:"environment,omitempty"`
	User          string   `yaml:"user" json:"user,omitempty"`
	Actions       []Action `yaml:"actions" env-required:"true"`
}

type Action struct {
	On               string        `yaml:"on" env-default:"push" json:"on,omitempty"`
	Branch           string        `yaml:"branch" env-default:"master" json:"branch,omitempty"`
	Cwd              string        `yaml:"cwd" json:"cwd,omitempty"`
	User             string        `yaml:"user" json:"user,omitempty"`
	Script           string        `yaml:"script" json:"script,omitempty"`
	Run              []string      `yaml:"run" json:"run,omitempty"`
	Environment      EnvList       `yaml:"environment" json:"environment,omitempty"`
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

	if (cfg.Ssl.CertFilePath != "") != (cfg.Ssl.KeyFilePath != "") {
		return cfg, fmt.Errorf("ssl requires both `cert_file_path` and `key_file_path`")
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
	if cfg.MaxConcurrentActions <= 0 {
		return cfg, fmt.Errorf("'max_concurrent_actions' must be a positive integer")
	}

	if err := validateEnvEntries(cfg.Environment); err != nil {
		return cfg, fmt.Errorf("root environment: %w", err)
	}

	projectsWithDefaults, err := validateAndSetDefaultsConfigProjects(cfg.Projects, globalDefaults{
		Timeout:          cfg.ActionsTimeout,
		GracefulShutdown: cfg.ActionsGracefulShutdown,
	}, cfg.Environment, cfg.User)
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

type globalDefaults struct {
	Timeout          time.Duration
	GracefulShutdown time.Duration
}

func validateAndSetDefaultsConfigProjects(projects map[string]Project, global globalDefaults, rootEnv EnvList, rootUser string) (map[string]Project, error) {
	for projectName, project := range projects {
		if err := setDefaultAndCheckRequired(&project); err != nil {
			return nil, fmt.Errorf("project %q has issue with its fields: %w", projectName, err)
		}

		if err := isValidProjectName(projectName); err != nil {
			return nil, fmt.Errorf("bad project name %q: %w", projectName, err)
		}

		if err := validateEnvEntries(project.Environment); err != nil {
			return nil, fmt.Errorf("project %q environment: %w", projectName, err)
		}

		if len(project.Actions) == 0 {
			return nil, fmt.Errorf(
				"project %q has no associated actions and can not be executed; "+
					"either add 'actions' list to the project or comment the project out",
				projectName,
			)
		}

		// The action's base env is the root layered with this project's own
		// entries; each action then appends its own on top (see below).
		projectEnv := slices.Concat(rootEnv, project.Environment)
		// Same override chain for the user: this project's value wins over the
		// root, and each action may still override it below.
		projectUser := project.User
		if projectUser == "" {
			projectUser = rootUser
		}
		actionsWithDefaults, err := validateAndSetDefaultConfigActions(projectName, project.Actions, global, projectEnv, projectUser)
		if err != nil {
			return nil, fmt.Errorf("action validation failed: %w", err)
		}
		project.Actions = actionsWithDefaults
		projects[projectName] = project
	}
	return projects, nil
}

func validateAndSetDefaultConfigActions(projectName string, actions []Action, global globalDefaults, projectEnv EnvList, projectUser string) ([]Action, error) {
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

		if err := validateEnvEntries(action.Environment); err != nil {
			return nil, wrapActionErr(err)
		}

		if action.User == "" {
			action.User = projectUser
		}

		if runtime.GOOS != "windows" && action.User != "" {
			if _, err := user.Lookup(action.User); err != nil {
				return nil, wrapActionErr(fmt.Errorf("has a user field = %q, but this user can't be found: %w", action.User, err))
			}
		}

		if action.Timeout == 0 {
			action.Timeout = global.Timeout
		} else if action.Timeout < 0 {
			return nil, wrapActionErr(fmt.Errorf("'timeout' cannot be a negative value"))
		}
		if action.GracefulShutdown == 0 {
			action.GracefulShutdown = global.GracefulShutdown
		} else if action.GracefulShutdown < 0 {
			return nil, wrapActionErr(fmt.Errorf("'graceful_shutdown' cannot be a negative value"))
		}

		action.Environment = slices.Concat(projectEnv, action.Environment)

		actions[i] = action
	}
	return actions, nil
}

// validateEnvEntries checks that each action `environment` entry has the
// "KEY=VALUE" shape with a POSIX-conformant KEY. The VALUE itself is only
// resolved at run time (it depends on the process environment), so we don't
// interpolate here -- see the action runner's createEnv.
func validateEnvEntries(entries []string) error {
	for _, entry := range entries {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			return fmt.Errorf("invalid environment entry %q: expected \"KEY=VALUE\" form", entry)
		}
		if err := isValidEnvKey(key); err != nil {
			return fmt.Errorf("invalid environment entry %q: %w", entry, err)
		}
	}
	return nil
}

// isValidEnvKey enforces the POSIX name convention for env variables:
// a leading letter or underscore followed by letters, digits or underscores.
func isValidEnvKey(key string) error {
	if key == "" {
		return fmt.Errorf("env key can't be empty")
	}
	for i, r := range key {
		isDigit := r >= '0' && r <= '9'
		isAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		if !isAlpha && (!isDigit || i <= 0) {
			return fmt.Errorf("env key %q can only contain [A-Za-z0-9_] and can't start with a digit", key)
		}
	}
	return nil
}

// isValidProjectName checks if a name of a project is valid.
// As we're using project names directly in the url `/projects/:NAME` we
// shouldn't just allow anything here. Restricting to letters and some chars,
// but no consecutive ".." or "." chars so mux won't go crazy.
func isValidProjectName(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("project name can't be empty")
	}

	switch s[0] {
	case '.':
		return fmt.Errorf("name can't start with '.' symbol")
	}

	var lastRune rune
	for _, r := range s {
		if r == '.' && r == lastRune {
			return fmt.Errorf("name can't contain two or more consecutive '.' chars")
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.' {
			return fmt.Errorf("name can only contain chars from range [a-Z0-9._-]")
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
