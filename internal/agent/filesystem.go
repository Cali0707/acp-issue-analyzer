package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FilesystemManager struct {
	maxWriteBytes int64
}

func NewFilesystemManager(maxWriteBytes int64) *FilesystemManager {
	if maxWriteBytes <= 0 {
		maxWriteBytes = 1 << 20 // 1 MiB default
	}
	return &FilesystemManager{maxWriteBytes: maxWriteBytes}
}

// ValidatePath checks that path is absolute and resolves within the sandbox root.
func (fm *FilesystemManager) ValidatePath(sandbox, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute: %s", path)
	}

	// Resolve symlinks on the sandbox to get its canonical path.
	sandboxResolved, err := filepath.EvalSymlinks(sandbox)
	if err != nil {
		sandboxResolved = filepath.Clean(sandbox)
	}

	// For the target path, resolve symlinks on the longest existing prefix,
	// then append the remaining unresolved suffix. This handles cases where
	// parent directories don't exist yet (writes to new paths).
	resolved := resolveExistingPrefix(path)

	rel, err := filepath.Rel(sandboxResolved, resolved)
	if err != nil {
		return fmt.Errorf("path %s is outside sandbox %s", path, sandbox)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %s escapes sandbox %s", path, sandbox)
	}
	return nil
}

// resolveExistingPrefix resolves symlinks on the longest existing ancestor,
// then appends the remaining path components.
func resolveExistingPrefix(path string) string {
	cleaned := filepath.Clean(path)

	// Walk up until we find an existing directory
	existing := cleaned
	var suffix []string
	for {
		if _, err := os.Stat(existing); err == nil {
			break
		}
		suffix = append([]string{filepath.Base(existing)}, suffix...)
		parent := filepath.Dir(existing)
		if parent == existing {
			// Reached root without finding anything — just return cleaned path
			return cleaned
		}
		existing = parent
	}

	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return cleaned
	}
	return filepath.Join(append([]string{resolved}, suffix...)...)
}

// ReadTextFile reads a file within the sandbox, with optional line/limit support.
func (fm *FilesystemManager) ReadTextFile(sandbox, path string, line, limit *int) (string, error) {
	if err := fm.ValidatePath(sandbox, path); err != nil {
		return "", err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	content := string(b)
	if line != nil || limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if line != nil && *line > 0 {
			start = *line - 1
			if start > len(lines) {
				start = len(lines)
			}
		}
		end := len(lines)
		if limit != nil && *limit > 0 && start+*limit < end {
			end = start + *limit
		}
		content = strings.Join(lines[start:end], "\n")
	}

	return content, nil
}

// WriteTextFile writes content to a file within the sandbox, enforcing size limits.
func (fm *FilesystemManager) WriteTextFile(sandbox, path, content string) error {
	if err := fm.ValidatePath(sandbox, path); err != nil {
		return err
	}

	if int64(len(content)) > fm.maxWriteBytes {
		return fmt.Errorf("write exceeds max size: %d > %d bytes", len(content), fm.maxWriteBytes)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
