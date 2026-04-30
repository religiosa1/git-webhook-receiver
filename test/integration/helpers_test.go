//go:build integration

package integration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/cmd"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	handlers "github.com/religiosa1/git-webhook-receiver/internal/http/webhook_handlers"
	"github.com/religiosa1/git-webhook-receiver/internal/requestmock"
)

const (
	fixtureDir      = "../../internal/requestmock/captured-requests"
	defaultProject  = "testproj"
	defaultSecret   = "testsecret"
	listenWaitLimit = 2 * time.Second
)

type providerInfo struct {
	Name    string
	Repo    string
	Fixture string
}

var providers = map[string]providerInfo{
	"github": {Name: "github", Repo: "religiosa1/github-test", Fixture: "github.json"},
	"gitea":  {Name: "gitea", Repo: "religiosa/staticus", Fixture: "gitea.json"},
}

func init() {
	// Register a process-wide sink so the default SIGINT/SIGTERM disposition
	// (terminate) never applies to the test binary, even in the brief windows
	// before Serve has installed its own handler or after it has returned.
	sink := make(chan os.Signal, 1)
	signal.Notify(sink, syscall.SIGINT, syscall.SIGTERM)
}

type serverOpts struct {
	Provider      string
	Secret        string
	Authorization string
	Run           []string
	Script        string
	DisableAPI    bool
	APIUser       string
	APIPassword   string
}

type Option func(*serverOpts)

func WithProvider(p string) Option        { return func(o *serverOpts) { o.Provider = p } }
func WithSecret(s string) Option          { return func(o *serverOpts) { o.Secret = s } }
func WithAuthorization(s string) Option   { return func(o *serverOpts) { o.Authorization = s } }
func WithRun(args []string) Option        { return func(o *serverOpts) { o.Run = args; o.Script = "" } }
func WithScript(s string) Option          { return func(o *serverOpts) { o.Script = s; o.Run = nil } }
func WithDisableAPI(b bool) Option        { return func(o *serverOpts) { o.DisableAPI = b } }
func WithBasicAuth(user, pass string) Option {
	return func(o *serverOpts) { o.APIUser = user; o.APIPassword = pass }
}

type testServer struct {
	BaseURL   string
	Provider  providerInfo
	Cfg       config.Config
	ActionsDB string
	Done      <-chan struct{}

	once sync.Once
}

func startServer(t *testing.T, opts ...Option) *testServer {
	t.Helper()

	o := serverOpts{
		Provider:   "github",
		Secret:     defaultSecret,
		Run:        []string{"env"},
		DisableAPI: true,
	}
	for _, opt := range opts {
		opt(&o)
	}

	prov, ok := providers[o.Provider]
	if !ok {
		t.Fatalf("unknown provider %q", o.Provider)
	}

	dir := t.TempDir()
	addr := pickFreePort(t)
	actionsDB := filepath.Join(dir, "actions.sqlite3")
	logsDB := filepath.Join(dir, "logs.sqlite3")
	cfgPath := filepath.Join(dir, "config.yml")

	yamlText := renderConfigYAML(addr, actionsDB, logsDB, prov, o)
	if err := os.WriteFile(cfgPath, []byte(yamlText), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v\n--- config ---\n%s", err, yamlText)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		cmd.Serve(cfg)
	}()

	baseURL := "http://" + addr
	waitListening(t, baseURL)

	s := &testServer{
		BaseURL:   baseURL,
		Provider:  prov,
		Cfg:       cfg,
		ActionsDB: actionsDB,
		Done:      done,
	}

	t.Cleanup(func() {
		s.shutdown(t)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("Serve did not exit within 10s after SIGINT")
		}
	})

	return s
}

func (s *testServer) shutdown(t *testing.T) {
	t.Helper()
	s.once.Do(func() {
		if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
			t.Fatalf("send SIGINT: %v", err)
		}
	})
}

func pickFreePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close port-picker listener: %v", err)
	}
	return addr
}

func waitListening(t *testing.T, baseURL string) {
	t.Helper()
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	deadline := time.Now().Add(listenWaitLimit)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start listening at %s within %s", baseURL, listenWaitLimit)
}

func renderConfigYAML(addr, actionsDB, logsDB string, prov providerInfo, o serverOpts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "addr: %q\n", addr)
	b.WriteString("log_level: error\n")
	b.WriteString("log_type: text\n")
	fmt.Fprintf(&b, "actions_db_file: %q\n", actionsDB)
	fmt.Fprintf(&b, "logs_db_file: %q\n", logsDB)
	fmt.Fprintf(&b, "disable_api: %t\n", o.DisableAPI)
	if o.APIUser != "" {
		fmt.Fprintf(&b, "api_user: %q\n", o.APIUser)
	}
	if o.APIPassword != "" {
		fmt.Fprintf(&b, "api_password: %q\n", o.APIPassword)
	}
	b.WriteString("projects:\n")
	fmt.Fprintf(&b, "  %s:\n", defaultProject)
	fmt.Fprintf(&b, "    git_provider: %s\n", prov.Name)
	fmt.Fprintf(&b, "    repo: %s\n", prov.Repo)
	if o.Secret != "" {
		fmt.Fprintf(&b, "    secret: %q\n", o.Secret)
	}
	if o.Authorization != "" {
		fmt.Fprintf(&b, "    authorization: %q\n", o.Authorization)
	}
	b.WriteString("    actions:\n")
	b.WriteString("      - on: push\n")
	b.WriteString("        branch: \"*\"\n")
	if len(o.Run) > 0 {
		runJSON, _ := json.Marshal(o.Run)
		fmt.Fprintf(&b, "        run: %s\n", runJSON)
	}
	if o.Script != "" {
		b.WriteString("        script: |\n")
		for _, line := range strings.Split(strings.TrimRight(o.Script, "\n"), "\n") {
			fmt.Fprintf(&b, "          %s\n", line)
		}
	}
	return b.String()
}

func signGitHub(secret string, body []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func signGitea(secret string, body []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func loadFixture(t *testing.T, fixtureFile string) (body []byte, headers http.Header) {
	t.Helper()
	rm := requestmock.LoadRequestMock(t, filepath.Join(fixtureDir, fixtureFile))
	headers = http.Header{}
	for k, v := range rm.Headers {
		headers.Set(k, v)
	}
	return []byte(rm.Body), headers
}

func loadGitHubFixture(t *testing.T) (body []byte, headers http.Header) {
	return loadFixture(t, "github.json")
}

func loadGiteaFixture(t *testing.T) (body []byte, headers http.Header) {
	return loadFixture(t, "gitea.json")
}

func postWebhook(t *testing.T, baseURL, project string, headers http.Header, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/projects/"+project, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, vv := range headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func parsePipeIDs(t *testing.T, resp *http.Response) []string {
	t.Helper()
	var out []handlers.ActionOutput
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode webhook response: %v", err)
	}
	ids := make([]string, len(out))
	for i, a := range out {
		ids[i] = a.PipeID
	}
	return ids
}

func waitForPipeline(t *testing.T, dbPath, pipeID string, timeout time.Duration) actiondb.PipeLineRecord {
	t.Helper()
	db, err := actiondb.New(dbPath, 0, 0)
	if err != nil {
		t.Fatalf("open actions db: %v", err)
	}
	defer db.Close()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := db.GetPipelineRecord(pipeID)
		if err == nil && rec.EndedAt.Valid {
			return rec
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("pipeline %q did not finish within %s", pipeID, timeout)
	return actiondb.PipeLineRecord{}
}

// adminGet builds a GET request to an admin endpoint. We always set
// Content-Type: application/json on these requests so that any future
// content-negotiation logic on the server side cannot silently break the
// tests by serving an unexpected representation. If creds is non-nil basic
// auth is attached.
type basicCreds struct{ User, Pass string }

func adminGet(t *testing.T, baseURL, path string, creds *basicCreds) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if creds != nil {
		req.SetBasicAuth(creds.User, creds.Pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// pipelineExists reports whether a record for pipeID is present at all
// (regardless of completion). Used by negative tests to confirm a rejected
// webhook never enqueued an action.
func pipelineExists(t *testing.T, dbPath, pipeID string) bool {
	t.Helper()
	db, err := actiondb.New(dbPath, 0, 0)
	if err != nil {
		t.Fatalf("open actions db: %v", err)
	}
	defer db.Close()
	_, err = db.GetPipelineRecord(pipeID)
	return err == nil
}
