package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_ValidPaths(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	// Create a subdirectory for existing path checks
	sub := filepath.Join(sandbox, "subdir")
	os.MkdirAll(sub, 0o755)

	tests := []struct {
		name string
		path string
	}{
		{"file in root", filepath.Join(sandbox, "file.txt")},
		{"file in subdir", filepath.Join(sub, "file.txt")},
		{"nested path", filepath.Join(sandbox, "a", "b", "c.txt")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := fm.ValidatePath(sandbox, tt.path); err != nil {
				t.Errorf("ValidatePath(%q) unexpected error: %v", tt.path, err)
			}
		})
	}
}

func TestValidatePath_EscapeSandbox(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	tests := []struct {
		name string
		path string
	}{
		{"parent traversal", filepath.Join(sandbox, "..", "etc", "passwd")},
		{"totally outside", "/etc/passwd"},
		{"sibling dir", filepath.Join(filepath.Dir(sandbox), "other", "file.txt")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fm.ValidatePath(sandbox, tt.path)
			if err == nil {
				t.Errorf("ValidatePath(%q) should reject path outside sandbox", tt.path)
			}
		})
	}
}

func TestValidatePath_RelativePath(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	err := fm.ValidatePath(sandbox, "relative/path.txt")
	if err == nil {
		t.Error("ValidatePath should reject relative paths")
	}
	if !strings.Contains(err.Error(), "must be absolute") {
		t.Errorf("error = %q, want 'must be absolute'", err.Error())
	}
}

func TestReadTextFile_Basic(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	path := filepath.Join(sandbox, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(path, []byte(content), 0o644)

	got, err := fm.ReadTextFile(sandbox, path, nil, nil)
	if err != nil {
		t.Fatalf("ReadTextFile() error: %v", err)
	}
	if got != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestReadTextFile_WithLineAndLimit(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	path := filepath.Join(sandbox, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\nline4\nline5"), 0o644)

	tests := []struct {
		name  string
		line  *int
		limit *int
		want  string
	}{
		{"no offset", nil, nil, "line1\nline2\nline3\nline4\nline5"},
		{"start at line 2", intPtr(2), nil, "line2\nline3\nline4\nline5"},
		{"start at line 3, limit 2", intPtr(3), intPtr(2), "line3\nline4"},
		{"limit only", nil, intPtr(3), "line1\nline2\nline3"},
		{"start past end", intPtr(100), nil, ""},
		{"line 1 limit 1", intPtr(1), intPtr(1), "line1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fm.ReadTextFile(sandbox, path, tt.line, tt.limit)
			if err != nil {
				t.Fatalf("ReadTextFile() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadTextFile_OutsideSandbox(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	_, err := fm.ReadTextFile(sandbox, "/etc/passwd", nil, nil)
	if err == nil {
		t.Error("ReadTextFile should reject paths outside sandbox")
	}
}

func TestReadTextFile_NonexistentFile(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	_, err := fm.ReadTextFile(sandbox, filepath.Join(sandbox, "nope.txt"), nil, nil)
	if err == nil {
		t.Error("ReadTextFile should error for nonexistent file")
	}
}

func TestWriteTextFile_Basic(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	path := filepath.Join(sandbox, "output.txt")
	err := fm.WriteTextFile(sandbox, path, "hello world")
	if err != nil {
		t.Fatalf("WriteTextFile() error: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "hello world" {
		t.Errorf("file content = %q", string(got))
	}
}

func TestWriteTextFile_CreatesDirectories(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	path := filepath.Join(sandbox, "a", "b", "c", "file.txt")
	err := fm.WriteTextFile(sandbox, path, "deep write")
	if err != nil {
		t.Fatalf("WriteTextFile() error: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "deep write" {
		t.Errorf("file content = %q", string(got))
	}
}

func TestWriteTextFile_OutsideSandbox(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(1 << 20)

	err := fm.WriteTextFile(sandbox, "/tmp/evil.txt", "data")
	if err == nil {
		t.Error("WriteTextFile should reject paths outside sandbox")
	}
}

func TestWriteTextFile_ExceedsMaxSize(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(10) // 10 bytes max

	path := filepath.Join(sandbox, "big.txt")
	err := fm.WriteTextFile(sandbox, path, "this is more than ten bytes")
	if err == nil {
		t.Error("WriteTextFile should reject content exceeding max size")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Errorf("error = %q, want 'exceeds max size'", err.Error())
	}

	// File should not have been created
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should not exist after rejected write")
	}
}

func TestWriteTextFile_ExactMaxSize(t *testing.T) {
	sandbox := t.TempDir()
	fm := NewFilesystemManager(5)

	path := filepath.Join(sandbox, "exact.txt")
	err := fm.WriteTextFile(sandbox, path, "12345")
	if err != nil {
		t.Fatalf("WriteTextFile() should allow content at exact max size: %v", err)
	}
}

func TestNewFilesystemManager_DefaultMaxBytes(t *testing.T) {
	fm := NewFilesystemManager(0)
	if fm.maxWriteBytes != 1<<20 {
		t.Errorf("maxWriteBytes = %d, want %d", fm.maxWriteBytes, 1<<20)
	}

	fm2 := NewFilesystemManager(-1)
	if fm2.maxWriteBytes != 1<<20 {
		t.Errorf("maxWriteBytes = %d, want %d", fm2.maxWriteBytes, 1<<20)
	}
}

func intPtr(v int) *int {
	return &v
}
