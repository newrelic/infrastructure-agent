// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build slow
// +build slow

package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Custom path mode redirects every agent file location (data, temp, integration binaries and
// configs, logging) via NRIA_* environment variables, as Agent Control does in production. This
// test builds the real newrelic-infra binary and runs it against a fake in-process collector to
// verify that, under a full AC-managed configuration:
//
//   - the agent's own reported inventory reflects the redirected paths;
//   - default telemetry (system, network, storage samples) keeps flowing;
//   - a real integration (nri-flex) configured under the custom plugin_dir reports its metrics;
//   - the default, unmanaged plugin directory is never scanned.
//
// cmd/newrelic-infra is package main, so it can't be imported directly; running the compiled
// binary is the only way to exercise its real startup path from a test.
const (
	maxNriFlexArchiveSize = 200 << 20 // 200MiB, generously above the actual ~14MiB release size.
	nriFlexBinaryName     = "nri-flex"

	customLocationAPIName   = "customLocationFlex"
	customLocationValue     = "custom-value"
	customLocationEventType = customLocationAPIName + "Sample"

	defaultLocationAPIName   = "defaultLocationFlex"
	defaultLocationValue     = "default-value"
	defaultLocationEventType = defaultLocationAPIName + "Sample"
)

var (
	buildBinaryOnce sync.Once
	agentBinaryPath string
	repoRootPath    string
	buildBinaryErr  error
)

// findRepoRoot resolves the repository root from the go.mod location.
func findRepoRoot() (string, error) {
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD: %w", err)
	}

	return filepath.Dir(strings.TrimSpace(string(out))), nil
}

// buildAgentBinary compiles the real newrelic-infra binary once per test run and returns its
// path.
func buildAgentBinary(t *testing.T) string {
	t.Helper()

	buildBinaryOnce.Do(func() {
		repoRoot, err := findRepoRoot()
		if err != nil {
			buildBinaryErr = err
			return
		}
		repoRootPath = repoRoot

		dir, err := os.MkdirTemp("", "newrelic-infra-bin")
		if err != nil {
			buildBinaryErr = err
			return
		}
		agentBinaryPath = filepath.Join(dir, "newrelic-infra")

		cmd := exec.Command("go", "build", "-o", agentBinaryPath, "./cmd/newrelic-infra")
		cmd.Dir = repoRoot

		if out, err := cmd.CombinedOutput(); err != nil {
			buildBinaryErr = fmt.Errorf("building newrelic-infra binary: %w\n%s", err, out)
		}
	})

	require.NoError(t, buildBinaryErr)

	return agentBinaryPath
}

// freeTCPPort returns an available local TCP port for the agent's status server to bind to.
func freeTCPPort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// nriFlexVersion returns the nri-flex version pinned in build/embed/integrations.version, the
// same file build/embed/integrations.mk reads for packaging.
func nriFlexVersion(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRootPath, "build", "embed", "integrations.version"))
	require.NoError(t, err)

	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "nri-flex,v"); ok {
			return rest
		}
	}

	t.Fatal("nri-flex version not found in build/embed/integrations.version")

	return ""
}

// downloadNriFlex fetches the nri-flex release binary from the official GitHub release used by
// build/embed/integrations.mk's get-nri-flex packaging target. It skips the test, rather than
// failing it, when the current platform has no nri-flex build or the binary can't be fetched.
func downloadNriFlex(t *testing.T) string {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skipf("nri-flex is only distributed as a linux binary, skipping on %s", runtime.GOOS)
	}

	version := nriFlexVersion(t)
	url := fmt.Sprintf(
		"https://github.com/newrelic/nri-flex/releases/download/v%s/nri-flex_linux_%s_%s.tar.gz",
		version, version, runtime.GOARCH,
	)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // url is built from a pinned, repo-controlled version.
	if err != nil {
		t.Skipf("could not reach nri-flex release URL %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("unexpected status %d downloading nri-flex from %s", resp.StatusCode, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer gz.Close()

	binPath := filepath.Join(t.TempDir(), nriFlexBinaryName)

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)

			break
		}

		if hdr.Name != nriFlexBinaryName {
			continue
		}

		out, err := os.OpenFile(binPath, os.O_CREATE|os.O_WRONLY, 0o755) //nolint:gosec // must be executable.
		require.NoError(t, err)

		written, err := io.Copy(out, io.LimitReader(tr, maxNriFlexArchiveSize+1))
		require.NoError(t, err)
		require.LessOrEqual(t, written, int64(maxNriFlexArchiveSize), "nri-flex binary exceeds the expected archive size")
		require.NoError(t, out.Close())

		return binPath
	}

	t.Skip("nri-flex binary not found in the downloaded archive")

	return ""
}

// syncBuffer is a concurrency-safe byte buffer. os/exec writes to cmd.Stdout/cmd.Stderr from a
// background goroutine for as long as the child process runs, so a plain bytes.Buffer isn't safe
// to read while that process is still active.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

type recordedRequest struct {
	path string
	body []byte
}

// fakeBackend is a minimal stand-in for the New Relic collector, covering identity connect,
// inventory delta ingest, legacy event/sample ingest, and command-channel polling.
type fakeBackend struct {
	mu   sync.Mutex
	reqs []recordedRequest
	srv  *httptest.Server
}

func newFakeBackend() *fakeBackend {
	fb := &fakeBackend{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", fb.handle)
	fb.srv = httptest.NewServer(mux)

	return fb
}

func (fb *fakeBackend) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	// The agent's default PayloadCompressionLevel is gzip level 6.
	if r.Header.Get("Content-Encoding") == "gzip" {
		if gz, err := gzip.NewReader(bytes.NewReader(body)); err == nil {
			if decoded, err := io.ReadAll(gz); err == nil {
				body = decoded
			}
			_ = gz.Close()
		}
	}

	switch {
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/identity/v1/connect"):
		fb.record(r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"identity":{"entityId":1,"GUID":"test-guid-1"}}`))
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/deltas"):
		fb.record(r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"payload":{}}`))
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/agent_commands"):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"return_value":[]}`))
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/events/bulk"):
		fb.record(r.URL.Path, body)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (fb *fakeBackend) record(path string, body []byte) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.reqs = append(fb.reqs, recordedRequest{path: path, body: body})
}

func (fb *fakeBackend) snapshot() []recordedRequest {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	out := make([]recordedRequest, len(fb.reqs))
	copy(out, fb.reqs)

	return out
}

func (fb *fakeBackend) close() {
	fb.srv.Close()
}

type rawDelta struct {
	Source string                 `json:"source"`
	Diff   map[string]interface{} `json:"diff"`
}

// extractDeltas parses both delta body shapes the agent may send: a single-entity object and a
// multi-entity array.
func extractDeltas(body []byte) []rawDelta {
	var bulk []struct {
		Deltas []rawDelta `json:"deltas"`
	}
	if err := json.Unmarshal(body, &bulk); err == nil {
		var all []rawDelta
		for _, b := range bulk {
			all = append(all, b.Deltas...)
		}

		if len(all) > 0 {
			return all
		}
	}

	var single struct {
		Deltas []rawDelta `json:"deltas"`
	}
	if err := json.Unmarshal(body, &single); err == nil {
		return single.Deltas
	}

	return nil
}

// findAgentConfigField returns the value of a field from the agent's own metadata/agent_config
// inventory delta, reported by pkg/plugins.AgentConfigPlugin from Config.PublicFields(). That
// plugin nests every field under a top-level "infrastructure" key in the delta diff.
func findAgentConfigField(reqs []recordedRequest, field string) (string, bool) {
	for _, r := range reqs {
		if !strings.Contains(r.path, "/deltas") {
			continue
		}

		for _, d := range extractDeltas(r.body) {
			if !strings.HasSuffix(d.Source, "agent_config") {
				continue
			}

			infra, ok := d.Diff["infrastructure"].(map[string]interface{})
			if !ok {
				continue
			}

			raw, ok := infra[field]
			if !ok {
				continue
			}

			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}

			if v, ok := m["value"]; ok {
				return fmt.Sprintf("%v", v), true
			}
		}
	}

	return "", false
}

// findConnectMetadataHostID returns the metadata["host.id"] field from the agent's identity
// connect request, populated from the NR_HOST_ID environment variable.
func findConnectMetadataHostID(reqs []recordedRequest) (string, bool) {
	for _, r := range reqs {
		if !strings.Contains(r.path, "/identity/v1/connect") {
			continue
		}

		var body struct {
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(r.body, &body); err != nil {
			continue
		}

		if v, ok := body.Metadata["host.id"]; ok && v != "" {
			return v, true
		}
	}

	return "", false
}

// allEvents flattens every event out of every recorded events/bulk request.
func allEvents(reqs []recordedRequest) []map[string]interface{} {
	var out []map[string]interface{}

	for _, r := range reqs {
		if !strings.Contains(r.path, "/events/bulk") {
			continue
		}

		var payload []struct {
			Events []map[string]interface{} `json:"Events"`
		}
		if err := json.Unmarshal(r.body, &payload); err != nil {
			continue
		}

		for _, p := range payload {
			out = append(out, p.Events...)
		}
	}

	return out
}

// eventType returns an event's type. Legacy OHI output (e.g. nri-flex) sets "event_type", while
// the agent's own native samples (pkg/sample.BaseEvent) only set the camelCase "eventType".
func eventType(e map[string]interface{}) string {
	if v, ok := e["event_type"]; ok {
		return fmt.Sprintf("%v", v)
	}

	return fmt.Sprintf("%v", e["eventType"])
}

// hasEvent reports whether any recorded events/bulk request contains an event with the given
// type and value.
func hasEvent(reqs []recordedRequest, wantType, value string) bool {
	for _, e := range allEvents(reqs) {
		if eventType(e) == wantType && fmt.Sprintf("%v", e["value"]) == value {
			return true
		}
	}

	return false
}

// hasEventType reports whether any recorded events/bulk request contains an event of the given
// type, regardless of its other fields.
func hasEventType(reqs []recordedRequest, wantType string) bool {
	for _, e := range allEvents(reqs) {
		if eventType(e) == wantType {
			return true
		}
	}

	return false
}

// writeFlexIntegration writes an nri-flex config that shells out to echo and splits the result
// into a "value" attribute, then wires it into an agent integration definition that points
// directly at the nri-flex binary. nri-flex reports apiName's event type with "Sample" appended.
func writeFlexIntegration(t *testing.T, nriFlexPath, dir, apiName, value string) {
	t.Helper()

	flexConfigPath := filepath.Join(dir, apiName+"-flex-config.yml")
	flexConfig := fmt.Sprintf(
		"name: %s\napis:\n  - name: %s\n    commands:\n      - run: echo \"value:%s\"\n        split_by: \":\"\n",
		apiName, apiName, value,
	)
	require.NoError(t, os.WriteFile(flexConfigPath, []byte(flexConfig), 0o600))

	definition := fmt.Sprintf("---\nintegrations:\n  - name: %s\n    exec: %s --config_path %s\n",
		apiName, nriFlexPath, flexConfigPath)
	require.NoError(t, os.WriteFile(filepath.Join(dir, apiName+".yaml"), []byte(definition), 0o600))
}

// pluginDirLogPattern matches the pluginDir=[...] field logged on the agent's "runtime
// configuration" line (cmd/newrelic-infra/newrelic-infra.go, logConfig).
var pluginDirLogPattern = regexp.MustCompile(`pluginDir="\[(.*?)\]"`)

// parsePluginInstanceDirs extracts cfg.PluginInstanceDirs, as reported live by the running agent
// on startup, from its stdout.
func parsePluginInstanceDirs(stdout string) ([]string, bool) {
	match := pluginDirLogPattern.FindStringSubmatch(stdout)
	if match == nil {
		return nil, false
	}

	if strings.TrimSpace(match[1]) == "" {
		return []string{}, true
	}

	return strings.Fields(match[1]), true
}

// waitUntil polls condition every interval until it returns true or the deadline elapses.
func waitUntil(deadline time.Time, interval time.Duration, condition func() bool) {
	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(interval)
	}
}

func TestCustomPathMode_RealBinary_AgentAndIntegrationsUnderACPaths(t *testing.T) {
	binaryPath := buildAgentBinary(t)
	nriFlexPath := downloadNriFlex(t)

	backend := newFakeBackend()
	defer backend.close()

	root := t.TempDir()
	agentDir := filepath.Join(root, "agent-dir")
	pluginDir := filepath.Join(root, "plugin-dir")
	customPluginInstallationDir := filepath.Join(root, "custom-plugin-installation-dir")
	agentTempDir := filepath.Join(root, "agent-temp-dir")
	loggingConfigsDir := filepath.Join(root, "logging-configs-dir")
	loggingHomeDir := filepath.Join(root, "logging-home-dir")
	defaultIntegrationsDir := filepath.Join(agentDir, "integrations.d")
	statusServerPort := freeTCPPort(t)

	const hostID = "test-host-id"

	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.MkdirAll(defaultIntegrationsDir, 0o755))
	require.NoError(t, os.MkdirAll(agentTempDir, 0o755))
	require.NoError(t, os.MkdirAll(customPluginInstallationDir, 0o755))
	require.NoError(t, os.MkdirAll(loggingConfigsDir, 0o755))
	require.NoError(t, os.MkdirAll(loggingHomeDir, 0o755))

	writeFlexIntegration(t, nriFlexPath, pluginDir, customLocationAPIName, customLocationValue)
	writeFlexIntegration(t, nriFlexPath, defaultIntegrationsDir, defaultLocationAPIName, defaultLocationValue)

	cfgPath := filepath.Join(root, "newrelic-infra.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("license_key: 0123456789012345678901234567890123456789\n"), 0o600))

	cmd := exec.Command(binaryPath, "--config", cfgPath)
	cmd.Env = append(os.Environ(),
		// Mirrors .fleetControl/agentControl/linux.yml's executable env block in full.
		"NRIA_AGENT_DIR="+agentDir,
		"NRIA_AGENT_TEMP_DIR="+agentTempDir,
		"NRIA_PLUGIN_DIR="+pluginDir,
		"NRIA_CUSTOM_PLUGIN_INSTALLATION_DIR="+customPluginInstallationDir,
		"NRIA_SAFE_BIN_DIR="+customPluginInstallationDir,
		"NRIA_LOGGING_CONFIGS_DIR="+loggingConfigsDir,
		"NRIA_LOGGING_HOME_DIR="+loggingHomeDir,
		"NRIA_STATUS_SERVER_ENABLED=true",
		fmt.Sprintf("NRIA_STATUS_SERVER_PORT=%d", statusServerPort),
		"NR_HOST_ID="+hostID,
		"NRIA_DISABLE_PLUGIN_DEFAULT_DIR_SCAN=true",
		"NRIA_COLLECTOR_URL="+backend.srv.URL,
		"NRIA_IDENTITY_URL="+backend.srv.URL,
		"NRIA_COMMAND_CHANNEL_URL="+backend.srv.URL,
		"NRIA_STARTUP_CONNECTION_TIMEOUT=5s",
		"NRIA_STARTUP_CONNECTION_RETRIES=3",
		"NRIA_LOG_LEVEL=debug",
		"NRIA_LOG_STDOUT=true",
	)

	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(t, cmd.Start())
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Logf("agent stdout:\n%s", stdout.String())
		t.Logf("agent stderr:\n%s", stderr.String())
	}()

	// ProcessSample is excluded: individual process events pass through a metric matcher that can
	// exclude any given process for reasons unrelated to custom-path mode.
	defaultSampleTypes := []string{"SystemSample", "NetworkSample", "StorageSample"}

	var reqs []recordedRequest

	waitUntil(time.Now().Add(90*time.Second), 200*time.Millisecond, func() bool {
		reqs = backend.snapshot()

		_, gotAgentConfig := findAgentConfigField(reqs, "plugin_dir")
		if !gotAgentConfig {
			return false
		}

		for _, sampleType := range defaultSampleTypes {
			if !hasEventType(reqs, sampleType) {
				return false
			}
		}

		return hasEvent(reqs, customLocationEventType, customLocationValue)
	})

	expectedAgentConfigFields := map[string]string{
		"agent_dir":                      agentDir,
		"agent_temp_dir":                 agentTempDir,
		"plugin_dir":                     pluginDir,
		"custom_plugin_installation_dir": customPluginInstallationDir,
		"safe_bin_dir":                   customPluginInstallationDir,
		"logging_configs_dir":            loggingConfigsDir,
		"logging_home_dir":               loggingHomeDir,
		"status_server_enabled":          "true",
		"status_server_port":             fmt.Sprintf("%d", statusServerPort),
	}
	for field, want := range expectedAgentConfigFields {
		got, ok := findAgentConfigField(reqs, field)
		require.True(t, ok, "expected an agent_config inventory delta reporting %s", field)
		assert.Equal(t, want, got, "agent_config field %s", field)
	}

	reportedHostID, ok := findConnectMetadataHostID(reqs)
	require.True(t, ok, "expected a connect request reporting metadata.host.id")
	assert.Equal(t, hostID, reportedHostID)

	for _, sampleType := range defaultSampleTypes {
		assert.True(t, hasEventType(reqs, sampleType),
			"expected a default %s to be reported under AC-managed paths", sampleType)
	}

	assert.True(t, hasEvent(reqs, customLocationEventType, customLocationValue),
		"expected the nri-flex integration configured via the custom AC plugin_dir to report")

	// A further grace window before the negative check: if the default integration were
	// mistakenly scanned, it would have already reported well within this time.
	time.Sleep(5 * time.Second)
	reqs = backend.snapshot()

	assert.False(t, hasEvent(reqs, defaultLocationEventType, defaultLocationValue),
		"integration configured in the default (unscanned) location must never report")

	gotPluginDirs, ok := parsePluginInstanceDirs(stdout.String())
	require.True(t, ok, `expected a "runtime configuration" log line reporting pluginDir`)
	assert.Equal(t, []string{pluginDir}, gotPluginDirs,
		"disable_plugin_default_dir_scan must leave only the custom plugin_dir in scope")
}
