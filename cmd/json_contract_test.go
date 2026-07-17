package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

const cliHelperEnv = "TNR_CLI_HELPER_PROCESS"

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv(cliHelperEnv) != "1" {
		return
	}

	separator := slices.Index(os.Args, "--")
	if separator == -1 {
		os.Exit(2)
	}
	os.Args = append([]string{"tnr"}, os.Args[separator+1:]...)
	os.Exit(Execute())
}

type cliResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runCLI(t *testing.T, extraEnv map[string]string, args ...string) cliResult {
	t.Helper()

	cmdArgs := []string{"-test.run=^TestCLIHelperProcess$", "--"}
	cmdArgs = append(cmdArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
	command.Env = append(os.Environ(),
		cliHelperEnv+"=1",
		"TNR_NO_SELFUPDATE=1",
		"TNR_HOME="+t.TempDir(),
		"TNR_API_TOKEN=",
		"TNR_API_URL=",
	)
	for key, value := range extraEnv {
		command.Env = append(command.Env, key+"="+value)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("CLI command timed out (possible interactive flow)\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running CLI helper: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}

	return cliResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}

func decodeSingleJSONDocument(t *testing.T, output string, target any) {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(output))
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout:\n%s", err, output)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout contains data after the JSON document: %v\nstdout:\n%s", err, output)
	}
}

func TestJSONContractForRootVersionAndParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{name: "root help", args: []string{"--json"}, wantExit: 0},
		{name: "root version flag", args: []string{"--version", "--json"}, wantExit: 0},
		{name: "version", args: []string{"version", "--json"}, wantExit: 0},
		{name: "version rejects positional", args: []string{"version", "--json", "extra"}, wantExit: 1},
		{name: "unknown command JSON first", args: []string{"--json", "not-a-command"}, wantExit: 1},
		{name: "unknown command JSON last", args: []string{"not-a-command", "--json"}, wantExit: 1},
		{name: "unknown flag JSON first", args: []string{"status", "--json", "--not-a-flag"}, wantExit: 1},
		{name: "unknown flag JSON last", args: []string{"status", "--not-a-flag", "--json"}, wantExit: 1},
		{name: "invalid JSON value", args: []string{"status", "--json=invalid"}, wantExit: 1},
		{name: "status rejects positional", args: []string{"status", "ignored", "--json"}, wantExit: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runCLI(t, nil, test.args...)
			if result.exitCode != test.wantExit {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, test.wantExit, result.stdout, result.stderr)
			}
			var document any
			decodeSingleJSONDocument(t, result.stdout, &document)
		})
	}
}

func TestJSONHelpDoesNotDuplicateFlags(t *testing.T) {
	result := runCLI(t, nil, "status", "--help", "--json")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, result.stdout, result.stderr)
	}

	var help struct {
		Flags []struct {
			Name string `json:"name"`
		} `json:"flags"`
	}
	decodeSingleJSONDocument(t, result.stdout, &help)

	seen := make(map[string]struct{}, len(help.Flags))
	for _, flag := range help.Flags {
		if _, duplicate := seen[flag.Name]; duplicate {
			t.Fatalf("flag %q appears more than once in JSON help", flag.Name)
		}
		seen[flag.Name] = struct{}{}
	}
}

func TestJSONConnectWithoutInstancesIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(server.Close)

	result := runCLI(t, map[string]string{
		"TNR_API_TOKEN": "test-token",
		"TNR_API_URL":   server.URL,
	}, "connect", "--json")
	if result.exitCode == 0 {
		t.Fatalf("connect unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}

	var response struct {
		Error string `json:"error"`
	}
	decodeSingleJSONDocument(t, result.stdout, &response)
	if !strings.Contains(response.Error, "no instances") {
		t.Fatalf("error = %q, want a no-instances error", response.Error)
	}
}

func TestJSONSnapshotCreateRequiresFlagsBeforeAuthentication(t *testing.T) {
	result := runCLI(t, nil, "snapshot", "create", "--json")
	if result.exitCode == 0 {
		t.Fatalf("snapshot create unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}

	var response struct {
		Error string `json:"error"`
	}
	decodeSingleJSONDocument(t, result.stdout, &response)
	if !strings.Contains(response.Error, "--instance-id") {
		t.Fatalf("error = %q, want missing --instance-id", response.Error)
	}
}

func TestJSONModifyKeepsEstimatedCostOffStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/list":
			_, _ = io.WriteString(w, `{"1":{"name":"test","status":"RUNNING","uuid":"uuid-1","storage":100,"cpuCores":"4","gpuType":"a6000","numGpus":"1"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v2/specs":
			_, _ = io.WriteString(w, `{"specs":{"a6000_x1":{"displayName":"A6000","gpuCount":1,"vcpuOptions":[4,6,8],"ramPerVCPUGiB":8,"storageGB":{"min":100,"max":500}}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v2/status":
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v2/pricing":
			_, _ = io.WriteString(w, `{"pricing":{"a6000_x1":0.5,"additional_vcpus":0.1,"disk_gb":0.001}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/1/modify":
			_, _ = io.WriteString(w, `{"identifier":"1","instance_name":"test"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	result := runCLI(t, map[string]string{
		"TNR_API_TOKEN": "test-token",
		"TNR_API_URL":   server.URL,
	}, "modify", "1", "--disk", "200", "--json")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, result.stdout, result.stderr)
	}
	if strings.Contains(result.stdout, "Estimated cost") {
		t.Fatalf("estimated cost leaked to stdout:\n%s", result.stdout)
	}
	if !strings.Contains(result.stderr, "Estimated cost") {
		t.Fatalf("estimated cost missing from stderr:\n%s", result.stderr)
	}
	var response struct {
		Identifier string `json:"identifier"`
	}
	decodeSingleJSONDocument(t, result.stdout, &response)
	if response.Identifier != "1" {
		t.Fatalf("identifier = %q, want 1", response.Identifier)
	}
}

func TestAdditionalCommandsHonorJSONContract(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{name: "completion", args: []string{"completion", "bash", "--json"}, wantExit: 0},
		{name: "login requires non-interactive credentials", args: []string{"login", "--json"}, wantExit: 1},
		{name: "logout", args: []string{"logout", "--json"}, wantExit: 0},
		{name: "ports help", args: []string{"ports", "--json"}, wantExit: 0},
		{name: "snapshot help", args: []string{"snapshot", "--json"}, wantExit: 0},
		{name: "self-update is rejected", args: []string{"update", "--json"}, wantExit: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runCLI(t, nil, test.args...)
			if result.exitCode != test.wantExit {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, test.wantExit, result.stdout, result.stderr)
			}
			var document any
			decodeSingleJSONDocument(t, result.stdout, &document)
		})
	}
}

func TestCommandsRejectUnexpectedPositionalsInJSONMode(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "create", args: []string{"create", "unexpected", "--json"}},
		{name: "status", args: []string{"status", "unexpected", "--json"}},
		{name: "connect", args: []string{"connect", "one", "two", "--json"}},
		{name: "delete", args: []string{"delete", "one", "two", "--json"}},
		{name: "modify", args: []string{"modify", "one", "two", "--json"}},
		{name: "login", args: []string{"login", "unexpected", "--json"}},
		{name: "logout", args: []string{"logout", "unexpected", "--json"}},
		{name: "update", args: []string{"update", "unexpected", "--json"}},
		{name: "completion", args: []string{"completion", "bash", "unexpected", "--json"}},
		{name: "ports", args: []string{"ports", "unexpected", "--json"}},
		{name: "ports list", args: []string{"ports", "list", "unexpected", "--json"}},
		{name: "ports forward", args: []string{"ports", "forward", "one", "two", "--json"}},
		{name: "snapshot", args: []string{"snapshot", "unexpected", "--json"}},
		{name: "snapshot list", args: []string{"snapshot", "list", "unexpected", "--json"}},
		{name: "snapshot create", args: []string{"snapshot", "create", "unexpected", "--json"}},
		{name: "snapshot delete", args: []string{"snapshot", "delete", "one", "two", "--json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runCLI(t, nil, test.args...)
			if result.exitCode == 0 {
				t.Fatalf("command unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
			}
			var response struct {
				Error string `json:"error"`
			}
			decodeSingleJSONDocument(t, result.stdout, &response)
			if response.Error == "" {
				t.Fatal("JSON error response has an empty error message")
			}
		})
	}
}
