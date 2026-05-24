package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyAcceptsValidDocumentation(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/README.md", `# Documentation

This page tells you where to start when you use the documentation.

## Start here

Use [Run the demo](./demo.md) to start the local stack.
`)
	writeDoc(t, root, "docs/demo.md", `# Run the demo

This how-to shows you how to start the local OpenBao observability stack.

## Before you begin

- Install Docker.

## Start the stack

Run the stack.

`+"```shell"+`
make compose-up
`+"```"+`

## Verify the result

Check that Grafana starts.
`)

	err := Verify(VerifyOptions{
		RepositoryRoot: root,
		DocsRoot:       filepath.Join(root, "docs"),
	})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}

func TestVerifyRejectsStyleAndStructureIssues(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/runbooks/bad.md", `# Bad runbook

Please just inspect this runbook — it has deterministic style issues.

## Confirm the issue

`+"```"+`
make verify
`+"```"+`
`)

	err := Verify(VerifyOptions{
		RepositoryRoot: root,
		DocsRoot:       filepath.Join(root, "docs"),
	})
	if err == nil {
		t.Fatal("Verify returned nil error")
	}

	message := err.Error()
	for _, want := range []string{
		"avoid banned style-guide phrase",
		"do not use em dashes",
		"fenced code blocks must include a language tag",
		"how-to and runbook pages must include ## Before you begin",
		"how-to and runbook pages must include ## Verify the result",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("Verify error missing %q:\n%s", want, message)
		}
	}
}

func TestVerifyRejectsBrokenInternalLinks(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/README.md", `# Documentation

This page links to another documentation page.

See [Missing page](./missing.md).
`)

	err := Verify(VerifyOptions{
		RepositoryRoot: root,
		DocsRoot:       filepath.Join(root, "docs"),
	})
	if err == nil {
		t.Fatal("Verify returned nil error")
	}
	if !strings.Contains(err.Error(), `internal link target "./missing.md" does not exist`) {
		t.Fatalf("Verify error did not mention broken link:\n%s", err)
	}
}

func writeDoc(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
