package config_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

func tmpConfigFile(t *testing.T, contents string) string {
	t.Helper()
	tmpdir := t.TempDir()
	tmpfile, err := os.CreateTemp(tmpdir, "config_*.yml")
	if err != nil {
		t.Fatalf("Unable to create temporary config for test: %s", err)
	}
	err = os.WriteFile(tmpfile.Name(), []byte(contents), 0o775)
	if err != nil {
		t.Fatalf("Unable to write config file %q contents: %s", tmpfile.Name(), err)
	}
	return tmpfile.Name()
}

func loadMockConfig(t *testing.T, contents string) config.Config {
	t.Helper()
	configFileName := tmpConfigFile(t, contents)
	cfg, err := config.Load(configFileName)
	if err != nil {
		t.Fatalf("Error loading the config file: %s", err)
	}
	return cfg
}

const testBaseProj = `projects:
  test-proj:
    git_provider: gitea
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]
`

func TestConfigLoad(t *testing.T) {
	t.Run("loads the test config", func(t *testing.T) {
		cfg := loadMockConfig(t, `
addr: "test.example.com:1234"
actions_db_file: db2.sqlite3 
`+testBaseProj)

		want := "test.example.com:1234"
		if cfg.Addr != want {
			t.Errorf("incorrect addr values read from config, want %s, got %s", want, cfg.Addr)
		}

		if want, got := "db2.sqlite3", cfg.ActionsDBFile; want != got {
			t.Errorf("incorrect actions db file read from config, want %s, got %s", want, got)
		}

		if l := len(cfg.Projects); l != 1 {
			t.Errorf("There must be only one project in config, but got %d", l)
		}

		project := cfg.Projects["test-proj"]

		if l := len(project.Actions); l != 1 {
			t.Fatalf("There must be only one action in test-proj in config, but got %d", l)
		}

		if want, got := "username/reponame", project.Repo; want != got {
			t.Errorf("incorrect repo value read from config, want %s, got %s", want, got)
		}
	})

	t.Run("sets the default values", func(t *testing.T) {
		cfg := loadMockConfig(t, testBaseProj)

		if want, got := "localhost:9090", cfg.Addr; want != got {
			t.Errorf("incorrect addr value read from config, want %s, got %s", want, got)
		}

		if want, got := "actions.sqlite3", cfg.ActionsDBFile; want != got {
			t.Errorf("incorrect actions db file read from config, want %s, got %s", want, got)
		}

		project := cfg.Projects["test-proj"]
		action := project.Actions[0]

		if want, got := "push", action.On; want != got {
			t.Errorf("Incorrect default on event, want %q, got %q", want, got)
		}

		if want, got := "master", action.Branch; want != got {
			t.Errorf("Incorrect default branch, want %q, got %q", want, got)
		}
	})
}

func TestConfigLoadEnv(t *testing.T) {
	configContents := testBaseProj

	t.Run("allows to override config values with env", func(t *testing.T) {
		overriddenAddr := "test3.example.com:32167"
		t.Setenv("ADDR", overriddenAddr)

		cfg := loadMockConfig(t, configContents)
		if cfg.Addr != overriddenAddr {
			t.Errorf("incorrect addr value read from config, want %s, got %s", overriddenAddr, cfg.Addr)
		}
	})

	t.Run("allows to override project values with env", func(t *testing.T) {
		secret := "test secret"
		auth := "test auth"
		dbFile := "db.sqlite3"
		t.Setenv("PROJECTS__test-proj__SECRET", secret)
		t.Setenv("PROJECTS__test-proj__AUTH", auth)
		t.Setenv("ACTIONS_DB_FILE", dbFile)

		cfg := loadMockConfig(t, configContents)
		project := cfg.Projects["test-proj"]

		if want, got := dbFile, cfg.ActionsDBFile; want != got {
			t.Errorf("incorrect action db file read from config, want %q, got %q", want, got)
		}

		if want, got := secret, project.Secret.RawContents(); want != got {
			t.Errorf("incorrect secret value read from config, want %q, got %q", want, got)
		}

		if want, got := auth, project.Authorization.RawContents(); want != got {
			t.Errorf("incorrect auth value read from config, want %q, got %q", want, got)
		}
	})

	t.Run("allows to set SSL CERT values with env", func(t *testing.T) {
		certFilePath := "testcertfile"
		keyFilePath := "testkeyfile"
		t.Setenv("SSL__CERT_FILE_PATH", certFilePath)
		t.Setenv("SSL__KEY_FILE_PATH", keyFilePath)

		config := loadMockConfig(t, configContents)

		if want, got := certFilePath, config.Ssl.CertFilePath; want != got {
			t.Errorf("incorrect Cert file path value read from config, want %q, got %q", want, got)
		}

		if want, got := keyFilePath, config.Ssl.KeyFilePath; want != got {
			t.Errorf("incorrect Key file path value read from config, want %q, got %q", want, got)
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
		{"can't contain 2 consecutive _", "bad__name"},
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

func TestDefaultMaxActionsStored(t *testing.T) {
	baseCfg := `
host: test.example.com
port: 1234
projects:
  test-proj:
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]
`
	makeConfig := func(maxActionsStored int) string {
		return baseCfg + fmt.Sprintf("\n"+"max_actions_stored: %d", maxActionsStored)
	}

	t.Run("default config value is 1_000", func(t *testing.T) {
		cfg := loadMockConfig(t, baseCfg)
		if cfg.MaxActionsStored != config.DefaultMaxActionsStored {
			t.Errorf("want %d, got %d", config.DefaultMaxActionsStored, cfg.MaxActionsStored)
		}
	})

	t.Run("explicitly passed values parsed", func(t *testing.T) {
		want := 42
		cfg := loadMockConfig(t, makeConfig(want))
		if cfg.MaxActionsStored != want {
			t.Errorf("want %d, got %d", want, cfg.MaxActionsStored)
		}
	})

	t.Run("explicitly passed zero will be interpreted as the default", func(t *testing.T) {
		cfg := loadMockConfig(t, makeConfig(0))
		if cfg.MaxActionsStored != config.DefaultMaxActionsStored {
			t.Errorf("want %d, got %d", config.DefaultMaxActionsStored, cfg.MaxActionsStored)
		}
	})
}

func TestDefaultTimeoutSeconds(t *testing.T) {
	baseCfg := `
host: test.example.com
port: 1234
projects:
  test-proj:
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]
`
	makeConfig := func(timeout string) string {
		return baseCfg + fmt.Sprintf("\n"+"actions_timeout: %s", timeout)
	}

	t.Run("default config value is DefaultTimeoutSeconds", func(t *testing.T) {
		cfg := loadMockConfig(t, baseCfg)
		if cfg.ActionsTimeout != config.DefaultTimeout {
			t.Errorf("want %s, got %s", config.DefaultTimeout, cfg.ActionsTimeout)
		}
	})

	t.Run("explicitly passed positive value is used", func(t *testing.T) {
		want := 120 * time.Second
		cfg := loadMockConfig(t, makeConfig("120s"))
		if cfg.ActionsTimeout != want {
			t.Errorf("want %s, got %s", want, cfg.ActionsTimeout)
		}
	})

	t.Run("zero falls back to default", func(t *testing.T) {
		cfg := loadMockConfig(t, makeConfig("0s"))
		if cfg.ActionsTimeout != config.DefaultTimeout {
			t.Errorf("want %s, got %s", config.DefaultTimeout, cfg.ActionsTimeout)
		}
	})

	t.Run("negative value is rejected", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig("-1s"))
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("expected error for default_timeout_seconds=-1, got nil")
		}
	})
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
			t.Errorf("Validation wasn't trigger when not expected")
		}
	})

	t.Run("empty log level results in info loglevel", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig(""))

		config, err := config.Load(configFileName)
		if err != nil {
			t.Error(err)
		}

		if want, got := "info", config.LogLevel; want != got {
			t.Errorf("Incorrect default loglevel, want %q, got %q", want, got)
		}
	})
}

func TestDefaultGracefulShutdownMS(t *testing.T) {
	baseCfg := `
host: test.example.com
port: 1234
projects:
  test-proj:
    repo: "username/reponame"
    actions:
      - run: ["node", "--version"]
`
	makeConfig := func(graceful_shutdown string) string {
		return baseCfg + fmt.Sprintf("\n"+"actions_graceful_shutdown: %s", graceful_shutdown)
	}

	t.Run("default config value is DefaultGracefulShutdownMS", func(t *testing.T) {
		cfg := loadMockConfig(t, baseCfg)
		if cfg.ActionsGracefulShutdown != config.DefaultGracefulShutdown {
			t.Errorf("want %s, got %s", config.DefaultGracefulShutdown, cfg.ActionsGracefulShutdown)
		}
	})

	t.Run("explicitly passed positive value is used", func(t *testing.T) {
		want := 5000 * time.Millisecond
		cfg := loadMockConfig(t, makeConfig("5000ms"))
		if cfg.ActionsGracefulShutdown != want {
			t.Errorf("want %s, got %s", want, cfg.ActionsGracefulShutdown)
		}
	})

	t.Run("zero falls back to default", func(t *testing.T) {
		cfg := loadMockConfig(t, makeConfig("0ms"))
		if cfg.ActionsGracefulShutdown != config.DefaultGracefulShutdown {
			t.Errorf("want %s, got %s", config.DefaultGracefulShutdown, cfg.ActionsGracefulShutdown)
		}
	})

	t.Run("negative value is rejected", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig("-1s"))
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("expected error for graceful_shutdown_ms=-1, got nil")
		}
	})
}

func TestTimeoutPropagationToActions(t *testing.T) {
	makeConfig := func(globalTimeout, actionTimeout time.Duration) string {
		action := "      - run: [\"node\", \"--version\"]"
		if actionTimeout != 0 {
			action += fmt.Sprintf("\n        timeout: %s", actionTimeout)
		}
		s := "projects:\n  test-proj:\n    repo: \"username/reponame\"\n    actions:\n" + action
		if globalTimeout != 0 {
			s = fmt.Sprintf("actions_timeout: %s\n", globalTimeout) + s
		}
		return s
	}

	t.Run("global timeout propagates to action when action has no timeout", func(t *testing.T) {
		want := 120 * time.Second
		cfg := loadMockConfig(t, makeConfig(want, 0))
		action := cfg.Projects["test-proj"].Actions[0]
		if action.Timeout != want {
			t.Errorf("want action timeout %s, got %s", want, action.Timeout)
		}
	})

	t.Run("action-specific timeout overrides global", func(t *testing.T) {
		globalTimeout := 120 * time.Second
		actionTimeout := 60 * time.Second
		cfg := loadMockConfig(t, makeConfig(globalTimeout, actionTimeout))
		action := cfg.Projects["test-proj"].Actions[0]
		if action.Timeout != actionTimeout {
			t.Errorf("want action timeout %s, got %s", actionTimeout, action.Timeout)
		}
	})

	t.Run("action inherits default global timeout when neither is set", func(t *testing.T) {
		cfg := loadMockConfig(t, makeConfig(0, 0))
		action := cfg.Projects["test-proj"].Actions[0]
		if action.Timeout != config.DefaultTimeout {
			t.Errorf("want action timeout %s, got %s", config.DefaultTimeout, action.Timeout)
		}
	})

	t.Run("negative action timeout is rejected", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig(0, -1))
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("expected error for action timeout_seconds=-1, got nil")
		}
	})
}

func TestGracefulShutdownPropagationToActions(t *testing.T) {
	makeConfig := func(global, local time.Duration) string {
		action := "      - run: [\"node\", \"--version\"]"
		if local != 0 {
			action += fmt.Sprintf("\n        graceful_shutdown: %s", local)
		}
		s := "projects:\n  test-proj:\n    repo: \"username/reponame\"\n    actions:\n" + action
		if global != 0 {
			s = fmt.Sprintf("actions_graceful_shutdown: %s\n", global) + s
		}
		return s
	}

	t.Run("global graceful_shutdown_ms propagates to action when action has none", func(t *testing.T) {
		want := 5000 * time.Millisecond
		cfg := loadMockConfig(t, makeConfig(want, 0))
		action := cfg.Projects["test-proj"].Actions[0]
		if action.GracefulShutdown != want {
			t.Errorf("want action graceful_shutdown %s, got %s", want, action.GracefulShutdown)
		}
	})

	t.Run("action-specific graceful_shutdown_ms overrides global", func(t *testing.T) {
		global := 5000 * time.Millisecond
		local := 1000 * time.Millisecond
		cfg := loadMockConfig(t, makeConfig(global, local))
		action := cfg.Projects["test-proj"].Actions[0]
		if action.GracefulShutdown != local {
			t.Errorf("want action graceful_shutdown %s, got %s", local, action.GracefulShutdown)
		}
	})

	t.Run("action inherits default global graceful_shutdown_ms when neither is set", func(t *testing.T) {
		cfg := loadMockConfig(t, makeConfig(0, 0))
		action := cfg.Projects["test-proj"].Actions[0]
		if action.GracefulShutdown != config.DefaultGracefulShutdown {
			t.Errorf("want action graceful_shutdown_ms %s, got %s", config.DefaultGracefulShutdown, action.GracefulShutdown)
		}
	})

	t.Run("negative action graceful_shutdown_ms is rejected", func(t *testing.T) {
		configFileName := tmpConfigFile(t, makeConfig(0, -1*time.Millisecond))
		if _, err := config.Load(configFileName); err == nil {
			t.Errorf("expected error for action graceful_shutdown_ms=-1, got nil")
		}
	})
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		input       string
		wantNetwork string
		wantAddress string
	}{
		{"localhost:9090", "tcp", "localhost:9090"},
		{"0.0.0.0:8080", "tcp", "0.0.0.0:8080"},
		{"unix:/tmp/webhook.sock", "unix", "/tmp/webhook.sock"},
		{"unix:///tmp/webhook.sock", "unix", "/tmp/webhook.sock"},
		{"unix://relative.sock", "unix", "relative.sock"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotNetwork, gotAddress := config.ParseAddr(tt.input)
			if gotNetwork != tt.wantNetwork {
				t.Errorf("network: want %q, got %q", tt.wantNetwork, gotNetwork)
			}
			if gotAddress != tt.wantAddress {
				t.Errorf("address: want %q, got %q", tt.wantAddress, gotAddress)
			}
		})
	}
}

func TestSslValidation(t *testing.T) {
	makeConfig := func(certFile, keyFile string) string {
		var sb strings.Builder
		sb.WriteString(testBaseProj)
		if certFile == "" && keyFile == "" {
			return sb.String()
		}
		sb.WriteString("ssl:\n")
		if certFile != "" {
			sb.WriteString("    cert_file_path: ")
			sb.WriteString(certFile)
			sb.WriteString("\n")
		}
		if keyFile != "" {
			sb.WriteString("    key_file_path: ")
			sb.WriteString(keyFile)
			sb.WriteString("\n")
		}

		return sb.String()
	}

	tests := []struct {
		name     string
		valid    bool
		certFile string
		keyFile  string
	}{
		{"no ssl is ok", true, "", ""},
		{"both cert and key files are ok", true, "foo", "bar"},
		{"only cert file is bad", false, "foo", ""},
		{"only key file is bad", false, "", "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFileName := tmpConfigFile(t, makeConfig(tt.certFile, tt.keyFile))
			_, err := config.Load(configFileName)
			if tt.valid {
				if err != nil {
					t.Errorf("expected to pass validation, but got err: %s", err)
				}
			} else {
				if err == nil {
					t.Error("expected validation to fail, but it passed")
				}
			}
		})
	}
}

func TestSensitiveDataMasking(t *testing.T) {
	makeTestCfg := func() config.Config {
		cfg := config.Config{
			Projects: make(map[string]config.Project),
		}
		cfg.APIPassword = "t3stPa55w0rd"
		cfg.Projects["proj1"] = config.Project{
			Authorization: "B3ar3rT0k3nV4lu3",
		}
		cfg.Projects["proj2"] = config.Project{
			Secret: "wh00kS3cr3tV4lu3",
		}
		return cfg
	}

	passwords := []string{"t3stPa55w0rd", "B3ar3rT0k3nV4lu3", "wh00kS3cr3tV4lu3"}

	t.Run("text handler masks sensitive data", func(t *testing.T) {
		cfg := makeTestCfg()
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		logger.Info("config", "config", cfg)
		output := buf.String()
		for _, pwd := range passwords {
			if strings.Contains(output, pwd) {
				t.Errorf("password %q leaked in text log output: %s", pwd, output)
			}
		}
	})

	t.Run("json handler masks sensitive data", func(t *testing.T) {
		cfg := makeTestCfg()
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		logger.Info("config", "config", cfg)
		output := buf.String()
		for _, pwd := range passwords {
			if strings.Contains(output, pwd) {
				t.Errorf("password %q leaked in JSON log output: %s", pwd, output)
			}
		}
	})

	t.Run("json.Marshal masks sensitive data", func(t *testing.T) {
		cfg := makeTestCfg()
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("json.Marshal failed: %s", err)
		}
		output := string(data)
		for _, pwd := range passwords {
			if strings.Contains(output, pwd) {
				t.Errorf("password %q leaked in json.Marshal output: %s", pwd, output)
			}
		}
	})
}
