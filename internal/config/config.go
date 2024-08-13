package config

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"unicode"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Host             string             `yaml:"host" env:"HOST" env-default:"localhost"`
	Port             int16              `yaml:"port" env:"PORT" env-default:"9090"`
	LogLevel         string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogFile          string             `yaml:"log_file" env:"LOG_FILE"`
	Ssl              SslConfig          `yaml:"ssl" env-prefix:"SSL__"`
	ActionsOutputDir string             `yaml:"actions_output_dir"`
	Projects         map[string]Project `yaml:"projects" env-required:"true"`
}

type SslConfig struct {
	CertFilePath string `yaml:"cert_file_path" env:"CERT_FILE_PATH"`
	KeyFilePath  string `yaml:"key_file_path" env:"KEY_FILE_PATH"`
}

// For both Projects and Actions only the fileds marked with env:"..." struct
// tag can be set through the env variables. See [applyEnvToProjectAndActions]

type Project struct {
	GitProvider   string   `yaml:"git_provider" env-default:"github"`
	Repo          string   `yaml:"repo" env-required:"true"`
	Authorization string   `yaml:"authorization" env:"AUTH"`
	Secret        string   `yaml:"secret" env:"SECRET"`
	Actions       []Action `yaml:"actions" env-required:"true"`
}

type Action struct {
	On     string   `yaml:"on" env-default:"push"`
	Branch string   `yaml:"branch" env-default:"master"`
	Cwd    string   `yaml:"cwd"`
	User   string   `yaml:"user"`
	Script string   `yaml:"script"`
	Run    []string `yaml:"run"`
}

func Load(configPath string) (Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return cfg, fmt.Errorf("error loading configuration %s: %w", configPath, err)
	}
	applyEnvToProjectAndActions(&cfg)

	if cfg.ActionsOutputDir != "" {
		err := os.MkdirAll(cfg.ActionsOutputDir, os.ModePerm)
		if err != nil {
			return cfg, fmt.Errorf("error creating actions output directory: %w", err)
		}
	}

	switch cfg.LogLevel {
	case "":
		cfg.LogLevel = "info"
	case "info", "debug", "warn", "error":
		// everything is ok, no action needed
	default:
		return cfg, fmt.Errorf("incorrect LogLevel value '%s'. Possible values are 'debug', 'info', 'warn', and 'error", cfg.LogLevel)
	}

	projectsWithDefaults, err := validateAndSetDefaultsConfigProjects(cfg.Projects)
	if err != nil {
		return cfg, fmt.Errorf("config's projects validation failed: %w", err)
	}

	cfg.Projects = projectsWithDefaults

	return cfg, nil
}

func validateAndSetDefaultsConfigProjects(projects map[string]Project) (map[string]Project, error) {
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

		actionsWithDefaults, err := validateAndSetDefaultConfigActions(projectName, project.Actions)
		if err != nil {
			return nil, fmt.Errorf("action validation failed: %w", err)
		}
		project.Actions = actionsWithDefaults
		projects[projectName] = project
	}
	return projects, nil
}

func validateAndSetDefaultConfigActions(projectName string, actions []Action) ([]Action, error) {
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
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return fmt.Errorf("name can only contain chars from range [a-Z0-9_-]")
		}
		lastRune = r
	}

	return nil
}
