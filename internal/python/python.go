package python

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

//go:embed scripts/*
var scriptFiles embed.FS

const uvSyncTimeout = 10 * time.Minute

// FindUV locates the uv binary in PATH.
func FindUV() (string, error) {
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return "", fmt.Errorf(
			"uv not found in PATH: install it from https://docs.astral.sh/uv/getting-started/installation/",
		)
	}
	return uvPath, nil
}

// EnsureEnvironment extracts embedded Python scripts to cacheDir/python/ and
// runs uv sync to install dependencies. Returns the project directory.
func EnsureEnvironment(ctx context.Context, uvPath, cacheDir string) (string, error) {
	projectDir := filepath.Join(cacheDir, "python")

	if err := extractScripts(projectDir); err != nil {
		return "", fmt.Errorf("failed to extract Python scripts: %w", err)
	}

	if err := uvSync(ctx, uvPath, projectDir); err != nil {
		return "", fmt.Errorf("failed to sync Python environment: %w", err)
	}

	return projectDir, nil
}

// RunScript builds an exec.Cmd that runs a Python script via uv.
func RunScript(ctx context.Context, uvPath, projectDir, scriptName string, args ...string) *exec.Cmd {
	scriptPath := filepath.Join(projectDir, scriptName)

	cmdArgs := []string{
		"run",
		"--project", projectDir,
		"--quiet",
		"python", scriptPath,
	}
	cmdArgs = append(cmdArgs, args...)

	return exec.CommandContext(ctx, uvPath, cmdArgs...)
}

func extractScripts(projectDir string) error {
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	return fs.WalkDir(scriptFiles, "scripts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel("scripts", path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(projectDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		content, err := scriptFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		return os.WriteFile(targetPath, content, 0o644)
	})
}

func uvSync(ctx context.Context, uvPath, projectDir string) error {
	ctx, cancel := context.WithTimeout(ctx, uvSyncTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, uvPath, "sync", "--project", projectDir, "--quiet")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("uv sync timed out (this may happen on first run while installing torch)")
		}
		return fmt.Errorf("uv sync failed: %w", err)
	}

	return nil
}
