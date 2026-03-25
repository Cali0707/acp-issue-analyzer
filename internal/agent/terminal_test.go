package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/cmurray/acp-issue-analyzer/internal/config"
)

func defaultAllowlist() []config.AllowedCommand {
	return []config.AllowedCommand{
		{Cmd: "echo", AllowAll: true},
		{Cmd: "cat", AllowAll: true},
		{Cmd: "sleep", AllowAll: true},
		{Cmd: "sh", AllowAll: true},
		{Cmd: "false", AllowAll: true},
		{Cmd: "git", ArgsPrefix: []string{"status"}},
		{Cmd: "git", ArgsPrefix: []string{"diff"}},
		{Cmd: "ls", AllowAll: true},
	}
}

// --- IsCommandAllowed tests ---

func TestIsCommandAllowed_AllowAll(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	if !tm.IsCommandAllowed("echo", []string{"hello"}) {
		t.Error("echo should be allowed with allow_all")
	}
}

func TestIsCommandAllowed_AllowAllNoArgs(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	if !tm.IsCommandAllowed("echo", nil) {
		t.Error("echo with no args should be allowed")
	}
}

func TestIsCommandAllowed_PrefixMatch(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	if !tm.IsCommandAllowed("git", []string{"status"}) {
		t.Error("git status should be allowed")
	}
	if !tm.IsCommandAllowed("git", []string{"status", "--short"}) {
		t.Error("git status --short should be allowed (prefix match)")
	}
	if !tm.IsCommandAllowed("git", []string{"diff", "HEAD"}) {
		t.Error("git diff HEAD should be allowed")
	}
}

func TestIsCommandAllowed_PrefixMismatch(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	if tm.IsCommandAllowed("git", []string{"push"}) {
		t.Error("git push should NOT be allowed")
	}
	if tm.IsCommandAllowed("git", []string{"checkout"}) {
		t.Error("git checkout should NOT be allowed")
	}
}

func TestIsCommandAllowed_PrefixNotEnoughArgs(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	if tm.IsCommandAllowed("git", nil) {
		t.Error("git with no args should NOT be allowed (needs prefix)")
	}
	if tm.IsCommandAllowed("git", []string{}) {
		t.Error("git with empty args should NOT be allowed")
	}
}

func TestIsCommandAllowed_UnknownCommand(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	if tm.IsCommandAllowed("rm", []string{"-rf", "/"}) {
		t.Error("rm should NOT be allowed")
	}
	if tm.IsCommandAllowed("curl", []string{"https://evil.com"}) {
		t.Error("curl should NOT be allowed")
	}
}

func TestIsCommandAllowed_EmptyAllowlist(t *testing.T) {
	tm := NewTerminalManager(nil)
	if tm.IsCommandAllowed("echo", []string{"hello"}) {
		t.Error("nothing should be allowed with empty allowlist")
	}
}

func TestIsCommandAllowed_MultiPrefixRule(t *testing.T) {
	tm := NewTerminalManager([]config.AllowedCommand{
		{Cmd: "docker", ArgsPrefix: []string{"compose", "up"}},
	})
	if !tm.IsCommandAllowed("docker", []string{"compose", "up"}) {
		t.Error("docker compose up should be allowed")
	}
	if !tm.IsCommandAllowed("docker", []string{"compose", "up", "-d"}) {
		t.Error("docker compose up -d should be allowed (extra args ok)")
	}
	if tm.IsCommandAllowed("docker", []string{"compose", "down"}) {
		t.Error("docker compose down should NOT be allowed")
	}
	if tm.IsCommandAllowed("docker", []string{"compose"}) {
		t.Error("docker compose alone should NOT be allowed (not enough args)")
	}
}

// --- Create tests ---

func TestCreate_Success(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("echo", []string{"hello world"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if id == "" {
		t.Fatal("Create() returned empty id")
	}

	// Wait for it to finish
	exitCode, _, err := tm.WaitForExit(id)
	if err != nil {
		t.Fatalf("WaitForExit() error: %v", err)
	}
	if exitCode == nil || *exitCode != 0 {
		t.Errorf("exit code = %v, want 0", exitCode)
	}

	output, truncated, _, _, err := tm.GetOutput(id)
	if err != nil {
		t.Fatalf("GetOutput() error: %v", err)
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("output = %q, want to contain 'hello world'", output)
	}
	if truncated {
		t.Error("output should not be truncated")
	}
}

func TestCreate_BlockedCommand(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	_, err := tm.Create("rm", []string{"-rf", "/"}, nil)
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error = %q, want to contain 'not allowed'", err.Error())
	}
}

func TestCreate_WithCwd(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	cwd := t.TempDir()
	id, err := tm.Create("ls", nil, &cwd)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	_, _, err = tm.WaitForExit(id)
	if err != nil {
		t.Fatalf("WaitForExit() error: %v", err)
	}
}

func TestCreate_NonZeroExit(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("false", nil, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	exitCode, _, err := tm.WaitForExit(id)
	if err != nil {
		t.Fatalf("WaitForExit() error: %v", err)
	}
	if exitCode == nil {
		t.Fatal("exitCode should not be nil")
	}
	if *exitCode == 0 {
		t.Error("exit code should be non-zero for `false`")
	}
}

func TestCreate_CapturesStderr(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("sh", []string{"-c", "echo errout >&2"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	tm.WaitForExit(id)

	output, _, _, _, err := tm.GetOutput(id)
	if err != nil {
		t.Fatalf("GetOutput() error: %v", err)
	}
	if !strings.Contains(output, "errout") {
		t.Errorf("output = %q, want to contain stderr 'errout'", output)
	}
}

func TestCreate_InvalidCommand(t *testing.T) {
	tm := NewTerminalManager([]config.AllowedCommand{
		{Cmd: "nonexistent_binary_xyz", AllowAll: true},
	})
	_, err := tm.Create("nonexistent_binary_xyz", nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

// --- GetOutput edge cases ---

func TestGetOutput_UnknownTerminal(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	_, _, _, _, err := tm.GetOutput("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown terminal ID")
	}
}

func TestGetOutput_BeforeExit(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("sh", []string{"-c", "echo before; sleep 60"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Give it a moment to produce output
	time.Sleep(200 * time.Millisecond)

	output, _, exitCode, _, err := tm.GetOutput(id)
	if err != nil {
		t.Fatalf("GetOutput() error: %v", err)
	}
	if !strings.Contains(output, "before") {
		t.Errorf("output = %q, want to contain 'before'", output)
	}
	if exitCode != nil {
		t.Errorf("exitCode should be nil while still running, got %d", *exitCode)
	}

	// Clean up: Kill first (handles SIGTERM->SIGKILL), then Release
	tm.Kill(id)
	tm.Release(id)
}

// --- WaitForExit ---

func TestWaitForExit_UnknownTerminal(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	_, _, err := tm.WaitForExit("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown terminal ID")
	}
}

// --- Kill ---

func TestKill_RunningProcess(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("sleep", []string{"60"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	err = tm.Kill(id)
	if err != nil {
		t.Fatalf("Kill() error: %v", err)
	}

	// Verify it actually exited
	exitCode, _, err := tm.WaitForExit(id)
	if err != nil {
		t.Fatalf("WaitForExit() error: %v", err)
	}
	// Killed processes have non-zero exit
	if exitCode != nil && *exitCode == 0 {
		t.Error("killed process should have non-zero exit code")
	}
}

func TestKill_UnknownTerminal(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	err := tm.Kill("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown terminal ID")
	}
}

func TestKill_AlreadyExited(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("echo", []string{"done"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	tm.WaitForExit(id)

	// Kill on an already-exited process should not error
	err = tm.Kill(id)
	if err != nil {
		t.Fatalf("Kill() on exited process error: %v", err)
	}
}

// --- Release ---

func TestRelease_RunningProcess(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("sleep", []string{"60"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	err = tm.Release(id)
	if err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	// Subsequent operations should fail - terminal removed
	_, _, _, _, err = tm.GetOutput(id)
	if err == nil {
		t.Fatal("expected error after Release")
	}
}

func TestRelease_AlreadyExited(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("echo", []string{"done"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	tm.WaitForExit(id)

	err = tm.Release(id)
	if err != nil {
		t.Fatalf("Release() on exited process error: %v", err)
	}
}

func TestRelease_UnknownTerminal(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	err := tm.Release("does-not-exist")
	if err != nil {
		t.Fatalf("Release() on unknown terminal should not error, got: %v", err)
	}
}

func TestRelease_DoubleRelease(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	id, err := tm.Create("echo", []string{"hi"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	tm.WaitForExit(id)

	if err := tm.Release(id); err != nil {
		t.Fatalf("first Release() error: %v", err)
	}
	if err := tm.Release(id); err != nil {
		t.Fatalf("second Release() should be idempotent, got: %v", err)
	}
}

// --- ReleaseAll ---

func TestReleaseAll(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())

	id1, _ := tm.Create("sleep", []string{"60"}, nil)
	id2, _ := tm.Create("sleep", []string{"60"}, nil)
	id3, _ := tm.Create("echo", []string{"quick"}, nil)
	tm.WaitForExit(id3) // let this one finish

	tm.ReleaseAll()

	// All terminals should be removed
	for _, id := range []string{id1, id2, id3} {
		_, _, _, _, err := tm.GetOutput(id)
		if err == nil {
			t.Errorf("expected error for terminal %s after ReleaseAll", id)
		}
	}
}

// --- Output truncation ---

func TestOutputTruncation(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	// Set a very small output limit
	tm.maxOutput = 32

	// Generate more output than the limit
	id, err := tm.Create("sh", []string{"-c", "dd if=/dev/zero bs=100 count=1 2>/dev/null | tr '\\0' 'A'"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	tm.WaitForExit(id)

	output, truncated, _, _, err := tm.GetOutput(id)
	if err != nil {
		t.Fatalf("GetOutput() error: %v", err)
	}
	if !truncated {
		t.Error("output should be truncated")
	}
	if len(output) > 32 {
		t.Errorf("output length = %d, should be <= 32", len(output))
	}
}

func TestOutputNotTruncatedWhenUnderLimit(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	tm.maxOutput = 1024

	id, err := tm.Create("echo", []string{"short"}, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	tm.WaitForExit(id)

	_, truncated, _, _, err := tm.GetOutput(id)
	if err != nil {
		t.Fatalf("GetOutput() error: %v", err)
	}
	if truncated {
		t.Error("output should NOT be truncated for small output")
	}
}

// --- limitedWriter unit tests ---

func TestLimitedWriter_ExactLimit(t *testing.T) {
	tp := &terminalProcess{maxBytes: 5}
	w := &limitedWriter{tp: tp}

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if tp.output.String() != "hello" {
		t.Errorf("output = %q", tp.output.String())
	}
	if tp.truncated {
		t.Error("should not be truncated at exact limit")
	}
}

func TestLimitedWriter_OverLimit(t *testing.T) {
	tp := &terminalProcess{maxBytes: 5}
	w := &limitedWriter{tp: tp}

	n, err := w.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 11 {
		t.Errorf("n = %d, want 11 (consumes all input)", n)
	}
	if tp.output.String() != "hello" {
		t.Errorf("output = %q, want 'hello'", tp.output.String())
	}
	if !tp.truncated {
		t.Error("should be truncated")
	}
}

func TestLimitedWriter_MultipleWrites(t *testing.T) {
	tp := &terminalProcess{maxBytes: 10}
	w := &limitedWriter{tp: tp}

	w.Write([]byte("aaa"))   // 3 bytes, total 3
	w.Write([]byte("bbbb"))  // 4 bytes, total 7
	w.Write([]byte("ccccc")) // 5 bytes, but only 3 should fit

	if tp.output.String() != "aaabbbbccc" {
		t.Errorf("output = %q, want 'aaabbbbccc'", tp.output.String())
	}
	if !tp.truncated {
		t.Error("should be truncated")
	}
}

func TestLimitedWriter_ZeroLimit(t *testing.T) {
	tp := &terminalProcess{maxBytes: 0}
	w := &limitedWriter{tp: tp}

	n, err := w.Write([]byte("anything"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 8 {
		t.Errorf("n = %d, want 8", n)
	}
	if tp.output.String() != "" {
		t.Errorf("output = %q, want empty", tp.output.String())
	}
	if !tp.truncated {
		t.Error("should be truncated with zero limit")
	}
}

func TestLimitedWriter_AlreadyTruncated(t *testing.T) {
	tp := &terminalProcess{maxBytes: 3}
	w := &limitedWriter{tp: tp}

	w.Write([]byte("abcde")) // first write exceeds
	w.Write([]byte("more"))  // second write, already truncated

	if tp.output.String() != "abc" {
		t.Errorf("output = %q, want 'abc'", tp.output.String())
	}
	if !tp.truncated {
		t.Error("should still be truncated")
	}
}

// --- Concurrent safety ---

func TestConcurrentCreateAndRelease(t *testing.T) {
	tm := NewTerminalManager(defaultAllowlist())
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			id, err := tm.Create("echo", []string{"concurrent"}, nil)
			if err != nil {
				return
			}
			tm.WaitForExit(id)
			tm.Release(id)
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
