package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Cali0707/baton/internal/config"
	"github.com/google/uuid"
)

type terminalProcess struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	output    bytes.Buffer
	truncated bool
	exited    bool
	exitCode  *int
	signal    *string
	maxBytes  int
	done      chan struct{}
}

type TerminalManager struct {
	mu        sync.Mutex
	terminals map[string]*terminalProcess
	allowlist []config.AllowedCommand
	maxOutput int
}

func NewTerminalManager(allowlist []config.AllowedCommand) *TerminalManager {
	return &TerminalManager{
		terminals: make(map[string]*terminalProcess),
		allowlist: allowlist,
		maxOutput: 1 << 20, // 1 MiB default output limit
	}
}

func (tm *TerminalManager) IsCommandAllowed(command string, args []string) bool {
	for _, ac := range tm.allowlist {
		if ac.Cmd != command {
			continue
		}
		if ac.AllowAll {
			return true
		}
		if len(ac.ArgsPrefix) > 0 && len(args) >= len(ac.ArgsPrefix) {
			match := true
			for i, prefix := range ac.ArgsPrefix {
				if args[i] != prefix {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func (tm *TerminalManager) Create(command string, args []string, cwd *string) (string, error) {
	if !tm.IsCommandAllowed(command, args) {
		return "", fmt.Errorf("command not allowed: %s %s", command, strings.Join(args, " "))
	}

	id := uuid.New().String()
	cmd := exec.Command(command, args...)
	if cwd != nil {
		cmd.Dir = *cwd
	}

	// Start in its own process group so we can kill the entire tree.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	tp := &terminalProcess{
		cmd:      cmd,
		maxBytes: tm.maxOutput,
		done:     make(chan struct{}),
	}

	cmd.Stdout = &limitedWriter{tp: tp}
	cmd.Stderr = &limitedWriter{tp: tp}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting command: %w", err)
	}

	go func() {
		err := cmd.Wait()
		tp.mu.Lock()
		defer tp.mu.Unlock()
		tp.exited = true
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				tp.exitCode = &code
			}
		} else {
			code := 0
			tp.exitCode = &code
		}
		close(tp.done)
	}()

	tm.mu.Lock()
	tm.terminals[id] = tp
	tm.mu.Unlock()

	return id, nil
}

func (tm *TerminalManager) GetOutput(id string) (output string, truncated bool, exitCode *int, signal *string, err error) {
	tm.mu.Lock()
	tp, ok := tm.terminals[id]
	tm.mu.Unlock()
	if !ok {
		return "", false, nil, nil, fmt.Errorf("terminal %s not found", id)
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()
	return tp.output.String(), tp.truncated, tp.exitCode, tp.signal, nil
}

func (tm *TerminalManager) WaitForExit(id string) (exitCode *int, signal *string, err error) {
	tm.mu.Lock()
	tp, ok := tm.terminals[id]
	tm.mu.Unlock()
	if !ok {
		return nil, nil, fmt.Errorf("terminal %s not found", id)
	}

	<-tp.done

	tp.mu.Lock()
	defer tp.mu.Unlock()
	return tp.exitCode, tp.signal, nil
}

// killProcessGroup sends a signal to the entire process group.
func killProcessGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func (tm *TerminalManager) Kill(id string) error {
	tm.mu.Lock()
	tp, ok := tm.terminals[id]
	tm.mu.Unlock()
	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}

	if tp.cmd.Process == nil {
		return nil
	}

	// Try SIGTERM to the process group first
	_ = killProcessGroup(tp.cmd.Process.Pid, syscall.SIGTERM)

	select {
	case <-tp.done:
		return nil
	case <-time.After(5 * time.Second):
	}

	// Force kill the process group
	_ = killProcessGroup(tp.cmd.Process.Pid, syscall.SIGKILL)
	<-tp.done
	return nil
}

func (tm *TerminalManager) Release(id string) error {
	tm.mu.Lock()
	tp, ok := tm.terminals[id]
	if !ok {
		tm.mu.Unlock()
		return nil
	}
	delete(tm.terminals, id)
	tm.mu.Unlock()

	if tp.cmd.Process != nil {
		select {
		case <-tp.done:
		default:
			// Kill entire process group to ensure child processes are reaped.
			_ = killProcessGroup(tp.cmd.Process.Pid, syscall.SIGKILL)
			<-tp.done
		}
	}

	return nil
}

func (tm *TerminalManager) ReleaseAll() {
	tm.mu.Lock()
	ids := make([]string, 0, len(tm.terminals))
	for id := range tm.terminals {
		ids = append(ids, id)
	}
	tm.mu.Unlock()

	for _, id := range ids {
		_ = tm.Release(id)
	}
}

type limitedWriter struct {
	tp *terminalProcess
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	w.tp.mu.Lock()
	defer w.tp.mu.Unlock()

	if w.tp.output.Len()+len(p) > w.tp.maxBytes {
		remaining := w.tp.maxBytes - w.tp.output.Len()
		if remaining > 0 {
			w.tp.output.Write(p[:remaining])
		}
		w.tp.truncated = true
		return len(p), nil // Still consume all input
	}
	w.tp.output.Write(p)
	return len(p), nil
}
