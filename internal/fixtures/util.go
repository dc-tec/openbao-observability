package fixtures

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func requireCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("missing required command %q: %w", name, err)
	}
	return nil
}

func combined(ctx context.Context, name string, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return out, exitError.ExitCode(), err
	}

	return out, -1, err
}

func readAndClose(responseBody interface {
	io.Reader
	io.Closer
}) ([]byte, error) {
	defer responseBody.Close()
	return io.ReadAll(responseBody)
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, normalizeTrailingNewline(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func normalizeTrailingNewline(content []byte) []byte {
	if len(content) == 0 || bytes.HasSuffix(content, []byte("\n")) {
		return content
	}
	return append(append([]byte(nil), content...), '\n')
}
