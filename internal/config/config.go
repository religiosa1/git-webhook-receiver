package config

import (
	"log"
	"os"
	"reflect"
	"regexp"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Host             string             `yaml:"host" env:"HOST" env-default:"localhost"`
	Port             int16              `yaml:"port" env:"PORT" env-default:"9090"`
	LogLevel         string             `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	LogFile          string             `yaml:"log_file" env:"LOG_FILE"`
	Ssl              SslConfig          `yaml:"ssl"`
	ActionsOutputDir string             `yaml:"actions_output_dir"`
	Projects         map[string]Project `yaml:"projects" env-required:"true"`
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
	On     string   `yaml:"on" env-default:"push"`
	Branch string   `yaml:"branch" env-default:"master"`
	Cwd    string   `yaml:"cwd"`
	User   string   `yaml:"user"`
	Script string   `yaml:"script"`
	Run    []string `yaml:"run"`
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

	if cfg.ActionsOutputDir != "" {
		err := os.MkdirAll(cfg.ActionsOutputDir, os.ModePerm)
		if err != nil {
			log.Fatalf("Error creating actions output directory: %s", err)
		}
	}

	projectNameRegex := regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	for projectName, project := range cfg.Projects {
		if errField := setDefaultAndCheckRequired(&project); errField != "" {
			log.Fatalf("Project '%s' doesn't have a value for field '%s' and it's a required field", projectName, errField)
		}

		if !projectNameRegex.MatchString(projectName) {
			log.Fatalf("'%s' is not a valid project name. Project can consist only of alphanumeric characters and symbols '_' and '-'", projectName)
		}

		if len(project.Actions) == 0 {
			log.Fatalf(
				"Project '%s' has no associated actions and can not be executed.\n"+
					"Either add 'actions' list to the project or comment the project out.",
				projectName,
			)
		}
		for i, action := range project.Actions {
			if errField := setDefaultAndCheckRequired(&action); errField != "" {
				log.Fatalf("Action %d (invoked on %s) of project '%s' doesn't have a value for field '%s' and it's a required field", i+1, action.On,
					projectName, errField)
			}
			if action.Script == "" && len(action.Run) == 00 {
				log.Fatalf(
					"Action %d (invoked on %s) of project '%s' has neither 'script' nor 'run' fields "+
						"and can not be executed", i+1, action.On,
					projectName,
				)
			}
			// if runtime.GOOS != "windows" && action.User != "" {
			// 	_, err := user.Lookup(action.User)
			// 	if err != nil {
			// 		log.Fatalf(
			// 			"Action %d (invoked on %s) of project '%s' has a user field = '%s', but this user can't be found: %s",
			// 			i+1, action.On, projectName, action.User, err,
			// 		)
			// 	}
			// }
			project.Actions[i] = action
		}
		cfg.Projects[projectName] = project
	}

	return &cfg
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
