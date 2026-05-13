package api

import (
	"strings"
	"testing"
)

func newTestConfig() *Config {
	cfg := &Config{
		AllowedCommands: []AllowedCommand{
			{
				ID:             "echo-hello",
				Cmd:            "/bin/sh",
				Args:           []string{"-c", "echo hello"},
				TimeoutSeconds: 5,
			},
			{
				ID:             "exit-3",
				Cmd:            "/bin/sh",
				Args:           []string{"-c", "exit 3"},
				TimeoutSeconds: 5,
			},
			{
				ID:             "sleep-long",
				Cmd:            "/bin/sh",
				Args:           []string{"-c", "sleep 5"},
				TimeoutSeconds: 1,
			},
			{
				ID:   "echo-default-timeout",
				Cmd:  "/bin/sh",
				Args: []string{"-c", "echo ok"},
				// TimeoutSeconds omitted → must default to 10.
			},
			{
				ID:             "stderr-writer",
				Cmd:            "/bin/sh",
				Args:           []string{"-c", "echo oops >&2; exit 1"},
				TimeoutSeconds: 5,
			},
		},
	}
	normalizeAllowedCommands(cfg.AllowedCommands)
	return cfg
}

func TestRunAllowedCommand_Success(t *testing.T) {
	cfg := newTestConfig()
	resp := runAllowedCommandFromConfig(cfg, "req-1", "echo-hello")

	if resp.ExitCode != 0 {
		t.Fatalf("expected exit_code 0, got %d (error=%q stderr=%q)", resp.ExitCode, resp.Error, resp.Stderr)
	}
	if resp.Error != "" {
		t.Fatalf("expected empty error, got %q", resp.Error)
	}
	if strings.TrimSpace(resp.Stdout) != "hello" {
		t.Fatalf("expected stdout 'hello', got %q", resp.Stdout)
	}
	if resp.RequestID != "req-1" {
		t.Fatalf("expected request_id echoed, got %q", resp.RequestID)
	}
	if resp.ID != "echo-hello" {
		t.Fatalf("expected id 'echo-hello', got %q", resp.ID)
	}
	if resp.Timestamp == "" {
		t.Fatalf("expected non-empty timestamp")
	}
}

func TestRunAllowedCommand_NonZeroExit(t *testing.T) {
	cfg := newTestConfig()
	resp := runAllowedCommandFromConfig(cfg, "", "exit-3")

	if resp.ExitCode != 3 {
		t.Fatalf("expected exit_code 3, got %d", resp.ExitCode)
	}
	if resp.Error != "" {
		t.Fatalf("expected empty error for non-zero exit, got %q", resp.Error)
	}
}

func TestRunAllowedCommand_StderrCaptured(t *testing.T) {
	cfg := newTestConfig()
	resp := runAllowedCommandFromConfig(cfg, "", "stderr-writer")

	if resp.ExitCode != 1 {
		t.Fatalf("expected exit_code 1, got %d", resp.ExitCode)
	}
	if strings.TrimSpace(resp.Stderr) != "oops" {
		t.Fatalf("expected stderr 'oops', got %q", resp.Stderr)
	}
}

func TestRunAllowedCommand_UnknownID(t *testing.T) {
	cfg := newTestConfig()
	resp := runAllowedCommandFromConfig(cfg, "req-2", "does-not-exist")

	if resp.ExitCode != -1 {
		t.Fatalf("expected exit_code -1, got %d", resp.ExitCode)
	}
	if resp.Error != "command not allowed: does-not-exist" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
	if resp.Stdout != "" || resp.Stderr != "" {
		t.Fatalf("expected empty stdout/stderr, got stdout=%q stderr=%q", resp.Stdout, resp.Stderr)
	}
	if resp.RequestID != "req-2" {
		t.Fatalf("expected request_id echoed back, got %q", resp.RequestID)
	}
}

func TestRunAllowedCommand_Timeout(t *testing.T) {
	cfg := newTestConfig()
	resp := runAllowedCommandFromConfig(cfg, "", "sleep-long")

	if resp.ExitCode != -1 {
		t.Fatalf("expected exit_code -1, got %d (error=%q)", resp.ExitCode, resp.Error)
	}
	if resp.Error != "timeout after 1s" {
		t.Fatalf("expected 'timeout after 1s', got %q", resp.Error)
	}
	// Duration should be roughly the timeout, give a generous upper bound to
	// avoid flakes on slow CI.
	if resp.DurationMs < 900 || resp.DurationMs > 5000 {
		t.Fatalf("unexpected duration_ms=%d for 1s timeout", resp.DurationMs)
	}
}

func TestExecuteCmdPayload_InvalidJSON(t *testing.T) {
	resp := executeCmdPayload([]byte("{not json"))

	if resp.ExitCode != -1 {
		t.Fatalf("expected exit_code -1, got %d", resp.ExitCode)
	}
	if !strings.HasPrefix(resp.Error, "invalid payload:") {
		t.Fatalf("expected 'invalid payload:' prefix, got %q", resp.Error)
	}
}

func TestExecuteCmdPayload_MissingID(t *testing.T) {
	resp := executeCmdPayload([]byte(`{"request_id":"x"}`))

	if resp.ExitCode != -1 {
		t.Fatalf("expected exit_code -1, got %d", resp.ExitCode)
	}
	if resp.Error != "invalid payload: missing id" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
	if resp.RequestID != "x" {
		t.Fatalf("expected request_id echoed back, got %q", resp.RequestID)
	}
}

func TestNormalizeTimeout(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, DefaultCommandTimeoutSeconds},
		{-5, DefaultCommandTimeoutSeconds},
		{1, 1},
		{10, 10},
		{300, 300},
		{301, MaxCommandTimeoutSeconds},
		{99999, MaxCommandTimeoutSeconds},
	}
	for _, c := range cases {
		if got := normalizeTimeout(c.in); got != c.want {
			t.Errorf("normalizeTimeout(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNormalizeAllowedCommands_AppliesDefaults(t *testing.T) {
	cfg := newTestConfig()
	for _, ac := range cfg.AllowedCommands {
		if ac.ID == "echo-default-timeout" && ac.TimeoutSeconds != DefaultCommandTimeoutSeconds {
			t.Fatalf("expected default timeout for 'echo-default-timeout', got %d", ac.TimeoutSeconds)
		}
	}
}

func TestFindAllowedCommandByID(t *testing.T) {
	cfg := newTestConfig()

	if got := FindAllowedCommandByID(cfg, "echo-hello"); got == nil || got.ID != "echo-hello" {
		t.Fatalf("expected to find 'echo-hello', got %+v", got)
	}
	if got := FindAllowedCommandByID(cfg, "nope"); got != nil {
		t.Fatalf("expected nil for unknown id, got %+v", got)
	}
	if got := FindAllowedCommandByID(nil, "echo-hello"); got != nil {
		t.Fatalf("expected nil for nil config, got %+v", got)
	}
}
