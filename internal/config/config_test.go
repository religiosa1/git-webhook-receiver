package config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

func tmpConfigFile(t *testing.T, contents string) string {
	t.Helper()
	tmpdir := t.TempDir()
	tmpfile, err := os.CreateTemp(tmpdir, "config_*.yml")
	if err != nil {
		t.Errorf("Unable to create temporary config for test: %s", err)
	}
	err = os.WriteFile(tmpfile.Name(), []byte(contents), 0o775)
	if err != nil {
		t.Errorf("Unable to write config file '%s' contents: %s", tmpfile.Name(), err)
	}
	return tmpfile.Name()
}

func TestConfigLoad(t *testing.T) {
	t.Run("loads the test config", func(t *testing.T) {
		configFileName := tmpConfigFile(t, `
host: test.example.com
port: 1234
projects:
  test-proj:
    git_provider: gitea
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]`,
		)

		config, err := config.Load(configFileName)
		if err != nil {
			t.Errorf("Error loading the config file: %s", err)
		}

		if l := len(config.Projects); l != 1 {
			t.Errorf("There must be only one project in config, but got %d", l)
		}

		project := config.Projects["test-proj"]

		if l := len(project.Actions); l != 1 {
			t.Errorf("There must be only one action in test-proj in config, but got %d", l)
		}

		if want, got := "username/reponame", project.Repo; want != got {
			t.Errorf("incorrect repo value read from config, want %s, got %s", want, got)
		}

		wantHost := "test.example.com"
		var wantPort int16 = 1234
		if config.Host != wantHost || config.Port != wantPort {
			t.Errorf("incorrect values read from config, want %s:%d, got %s:%d", wantHost, wantPort, config.Host, config.Port)
		}
	})

	t.Run("sets the default values", func(t *testing.T) {
		configFileName := tmpConfigFile(t, `
projects:
  test-proj:
    git_provider: gitea
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]`,
		)
		config, err := config.Load(configFileName)
		if err != nil {
			t.Error(err)
		}

		if want, got := "localhost", config.Host; want != got {
			t.Errorf("incorrect host value read from config, want %s, got %s", want, got)
		}

		var wantPort int16 = 9090
		if config.Port != wantPort {
			t.Errorf("incorrect port value read from config, want %d, got %d", wantPort, config.Port)
		}

		var project = config.Projects["test-proj"]
		var action = project.Actions[0]

		if want, got := "push", action.On; want != got {
			t.Errorf("Incorrect default on event, want '%s', got '%s'", want, got)
		}

		if want, got := "master", action.Branch; want != got {
			t.Errorf("Incorrect default branch, want '%s', got '%s'", want, got)
		}
	})
}

func TestConfigLoadEnv(t *testing.T) {
	configContents := `
projects:
  test-proj:
    git_provider: gitea
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]`

	t.Run("allows to override config values with env", func(t *testing.T) {
		overridenHost := "test2.example.com"
		var overridenPort int16 = 32167
		t.Setenv("HOST", overridenHost)
		t.Setenv("PORT", fmt.Sprintf("%d", overridenPort))

		configFileName := tmpConfigFile(t, configContents)
		config, err := config.Load(configFileName)
		if err != nil {
			t.Error(err)
		}

		if config.Host != overridenHost {
			t.Errorf("incorrect host value read from config, want %s, got %s", overridenHost, config.Host)
		}

		if config.Port != overridenPort {
			t.Errorf("incorrect port value read from config, want %d, got %d", overridenPort, config.Port)
		}
	})

	t.Run("allows to override project values with env", func(t *testing.T) {
		secret := "test secret"
		auth := "test auth"
		t.Setenv("PROJECTS__test-proj__SECRET", secret)
		t.Setenv("PROJECTS__test-proj__AUTH", auth)

		configFileName := tmpConfigFile(t, configContents)
		config, err := config.Load(configFileName)
		if err != nil {
			t.Error(err)
		}

		project := config.Projects["test-proj"]

		if want, got := secret, project.Secret; want != got {
			t.Errorf("incorrect secret value read from config, want '%s', got '%s'", want, got)
		}

		if want, got := auth, project.Authorization; want != got {
			t.Errorf("incorrect auth value read from config, want '%s', got '%s'", want, got)
		}
	})

	t.Run("allows to set SSL CERT values with env", func(t *testing.T) {
		certFilePath := "testcertfile"
		keyFilePath := "testkeyfile"
		t.Setenv("SSL__CERT_FILE_PATH", certFilePath)
		t.Setenv("SSL__KEY_FILE_PATH", keyFilePath)

		configFileName := tmpConfigFile(t, configContents)
		config, err := config.Load(configFileName)
		if err != nil {
			t.Error(err)
		}

		if want, got := certFilePath, config.Ssl.CertFilePath; want != got {
			t.Errorf("incorrect Cert file path value read from config, want '%s', got '%s'", want, got)
		}

		if want, got := keyFilePath, config.Ssl.KeyFilePath; want != got {
			t.Errorf("incorrect Key file path value read from config, want '%s', got '%s'", want, got)
		}
	})
}

func TestConfigRunScriptValidation(t *testing.T) {
	prelude := func(cont string) string {
		p := `
projects:
  test-proj:
    git_provider: gitea
    repo: "username/reponame"
    actions:
      - on: push`
		if cont == "" {
			return p
		}
		return p + "\n        " + cont
	}
	t.Run("either a run or script must be present in an action", func(t *testing.T) {
		configFileName := tmpConfigFile(t, prelude(""))

		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("Validation wasn't trigger on missing run and script in action")
		}

		configFileName = tmpConfigFile(t, prelude(`run: ["node", "--version"]`))
		if _, err := config.Load(configFileName); err != nil {
			t.Errorf("False positive in run validation: %s", err)
		}

		configFileName = tmpConfigFile(t, prelude(`script: "node --version"`))
		if _, err := config.Load(configFileName); err != nil {
			t.Errorf("False positive in script validation: %s", err)
		}
	})

	t.Run("project must have exclusively either run or script", func(t *testing.T) {
		configFileName := tmpConfigFile(t, prelude(`run: ["node", "--version"]
        script: "node --version"`))
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("Validation wasn't trigger on missing run and script in action")
		}
	})

	t.Run("config without any project isn't valid", func(t *testing.T) {
		configFileName := tmpConfigFile(t, `
host: test.example.com
port: 1234`,
		)
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("Validation wasn't trigger on missing run and script in action")
		}
	})

	t.Run("project without any actions isn't valid", func(t *testing.T) {
		configFileName := tmpConfigFile(t, `
host: test.example.com
port: 1234
  test-proj:
    git_provider: gitea
    repo: "username/reponame"`,
		)
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("Validation wasn't trigger on missing run and script in action")
		}
	})
}

func TestConfigProjectNameValidation(t *testing.T) {
	makeConfig := func(name string) string {
		return "projects:\n  " + name + ":\n" +
			`    git_provider: gitea
    repo: "username/reponame"
    actions:
      - on: push
        run: ["node", "--version"]`
	}
	t.Run("ok name is ok", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig("foo-project_bar2"))

		if _, err := config.Load(configFileName); err != nil {
			t.Error(err)
		}
	})

	badNames := []struct {
		desc string
		name string
	}{
		{"can't start with _", "_bad"},
		{"can't contain 2 consequitive _", "bad__name"},
		{"can't contain chars outside of ascii range", "a:b"},
	}
	for _, tt := range badNames {
		t.Run(tt.desc, func(t *testing.T) {
			configFileName := tmpConfigFile(t, makeConfig("foo-project_bar2"))

			if _, err := config.Load(configFileName); err != nil {
				t.Errorf("Validation wasn't trigger when expected")
			}
		})
	}
}

func TestConfigPossibleLogLevels(t *testing.T) {
	makeConfig := func(logLevel string) string {
		p := `
host: test.example.com
port: 1234
projects:
  test-proj:
    git_provider: gitea
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]`
		if logLevel == "" {
			return p
		}
		return p + "\nlog_level: " + logLevel
	}

	goodNames := []string{"debug", "info", "warn", "error"}

	for _, name := range goodNames {
		t.Run(fmt.Sprintf("testing LogLevel %s", name), func(t *testing.T) {
			configFileName := tmpConfigFile(t, makeConfig(name))

			if _, err := config.Load(configFileName); err != nil {
				t.Error(err)
			}
		})
	}

	t.Run("testing bad LogLevel", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig("warn2"))

		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("Validation was't trigger when not expected")
		}
	})

	t.Run("empty log level results in info loglevel", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig(""))

		config, err := config.Load(configFileName)
		if err != nil {
			t.Error(err)
		}

		if want, got := "info", config.LogLevel; want != got {
			t.Errorf("Incorrect default loglevel, want '%s', got '%s'", want, got)
		}
	})
}
