package config

import (
	"fmt"
	"os"
	"os/user"
	"reflect"
	"runtime"
	"unicode"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Host             string             `yaml:"host" env:"HOST" env-default:"localhost"`
	Port             int16              `yaml:"port" env:"PORT" env-default:"9090"`
	LogLevel         string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogFile          string             `yaml:"log_file" env:"LOG_FILE"`
	Ssl              SslConfig          `yaml:"ssl"`
	ActionsOutputDir string             `yaml:"actions_output_dir"`
	MaxOutputFiles   int                `yaml:"max_output_files" env-default:"10000"`
	Projects         map[string]Project `yaml:"projects" env-prefix:"projects__" env-required:"true"`
}

type SslConfig struct {
	CertFilePath string `yaml:"cert_file_path" env:"CERT_FILE_PATH"`
	KeyFilePath  string `yaml:"key_file_path" env:"KEY_FILE_PATH"`
}

type Project struct {
	GitProvider string   `yaml:"git_provider" env-default:"gitea"`
	Repo        string   `yaml:"repo" env-required:"true"`
	Actions     []Action `yaml:"actions" env-required:"true"`
}

type Action struct {
	On            string   `yaml:"on" env-default:"push"`
	Branch        string   `yaml:"branch" env-default:"master"`
	Authorization string   `yaml:"authorization" env:"AUTH"`
	Secret        string   `yaml:"secret" env:"SECRET"`
	Cwd           string   `yaml:"cwd"`
	User          string   `yaml:"user"`
	Script        string   `yaml:"script"`
	Run           []string `yaml:"run"`
}

func Load(configPath string) (*Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("error loading configuration %s: %w", configPath, err)
	}

	if cfg.ActionsOutputDir != "" {
		err := os.MkdirAll(cfg.ActionsOutputDir, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("error creating actions output directory: %w", err)
		}
	}

	switch cfg.LogLevel {
	case "":
		cfg.LogLevel = "info"
	case "info", "debug", "warn", "error":
		// everything is ok, no action needed
	default:
		return nil, fmt.Errorf("incorrect LogLevel value '%s'. Possible values are 'debug', 'info', 'warn', and 'error", cfg.LogLevel)
	}

	projectsWithDefaults, err := validateAndSetDefaultsConfigProjects(cfg.Projects)
	if err != nil {
		return nil, fmt.Errorf("config's projects validation failed: %w", err)
	}

	cfg.Projects = projectsWithDefaults

	return &cfg, nil
}

// cleanenv doesn't seem to respect its struct tags for map values, so we're setting them ourself
func setDefaultAndCheckRequired[T Project | Action](item *T) string {
	typesType := reflect.TypeOf(*item)
	typesValue := reflect.ValueOf(item).Elem()
	for i := 0; i < typesType.NumField(); i++ {
		field := typesType.Field(i)
		fieldValue := typesValue.Field(i)
		isRequired := field.Tag.Get("env-required") == "true"
		if fieldValue.Type().Kind() == reflect.String && fieldValue.String() == "" {
			defaultValue := field.Tag.Get("env-default")
			if defaultValue != "" {
				fieldValue.SetString(defaultValue)
			} else if isRequired {
				return field.Name
			}
		}
	}
	return ""
}

func validateAndSetDefaultsConfigProjects(projects map[string]Project) (map[string]Project, error) {
	for projectName, project := range projects {
		if errField := setDefaultAndCheckRequired(&project); errField != "" {
			return nil, fmt.Errorf("project '%s' doesn't have a value for field '%s' and it's a required field", projectName, errField)
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

		if errField := setDefaultAndCheckRequired(&action); errField != "" {
			return nil, wrapActionErr(fmt.Errorf("doesn't have a value for field '%s' and it's a required field", errField))
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
