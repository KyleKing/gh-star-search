package python

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFindUV(t *testing.T) {
	uvPath, err := FindUV()
	if err != nil {
		t.Skipf("uv not installed: %v", err)
	}

	if uvPath == "" {
		t.Error("FindUV returned empty path")
	}
}

func TestExtractScripts(t *testing.T) {
	projectDir := t.TempDir()

	if err := extractScripts(projectDir); err != nil {
		t.Fatalf("extractScripts failed: %v", err)
	}

	expectedFiles := []string{
		"pyproject.toml",
		"summarize.py",
		"embed.py",
	}

	for _, name := range expectedFiles {
		path := filepath.Join(projectDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not found: %v", name, err)
		}
	}
}

func TestExtractScripts_Idempotent(t *testing.T) {
	projectDir := t.TempDir()

	if err := extractScripts(projectDir); err != nil {
		t.Fatalf("first extraction failed: %v", err)
	}

	if err := extractScripts(projectDir); err != nil {
		t.Fatalf("second extraction failed: %v", err)
	}
}

func TestRunScript_BuildsCommand(t *testing.T) {
	cmd := RunScript(context.Background(), "/usr/bin/uv", "/tmp/project", "summarize.py", "--json", "--method", "auto")

	args := cmd.Args
	expected := []string{
		"/usr/bin/uv", "run",
		"--project", "/tmp/project",
		"--quiet",
		"python", "/tmp/project/summarize.py",
		"--json", "--method", "auto",
	}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}

	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("arg[%d]: expected %q, got %q", i, arg, args[i])
		}
	}
}

func TestEnsureEnvironment(t *testing.T) {
	uvPath, err := FindUV()
	if err != nil {
		t.Skipf("uv not installed: %v", err)
	}

	cacheDir := t.TempDir()

	projectDir, err := EnsureEnvironment(context.Background(), uvPath, cacheDir)
	if err != nil {
		t.Fatalf("EnsureEnvironment failed: %v", err)
	}

	expectedDir := filepath.Join(cacheDir, "python")
	if projectDir != expectedDir {
		t.Errorf("expected project dir %q, got %q", expectedDir, projectDir)
	}

	if _, err := os.Stat(filepath.Join(projectDir, "pyproject.toml")); err != nil {
		t.Errorf("pyproject.toml not found in project dir: %v", err)
	}
}
