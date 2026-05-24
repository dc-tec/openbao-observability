package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBundleIsDeterministic(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "README.md"), "OpenBao observability\n", 0o644)
	mustWriteFile(t, filepath.Join(root, "contracts", "metrics.yaml"), "metrics: []\n", 0o644)
	mustWriteFile(t, filepath.Join(root, "generated", "grafana", "overview.json"), "{}\n", 0o644)

	oldTime := time.Unix(42, 0)
	if err := os.Chtimes(filepath.Join(root, "README.md"), oldTime, oldTime); err != nil {
		t.Fatalf("change file time: %v", err)
	}

	first := filepath.Join(root, "dist", "first.tar.gz")
	second := filepath.Join(root, "dist", "second.tar.gz")
	opts := BundleOptions{
		RepositoryRoot:  root,
		Version:         "0.1.0",
		SourceDateEpoch: 1_700_000_000,
		Includes:        []string{"README.md", "contracts", "generated"},
	}
	opts.OutputPath = first
	if err := Bundle(opts); err != nil {
		t.Fatalf("bundle first archive: %v", err)
	}
	opts.OutputPath = second
	if err := Bundle(opts); err != nil {
		t.Fatalf("bundle second archive: %v", err)
	}

	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first archive: %v", err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second archive: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("bundle output differs for identical inputs")
	}

	entries := tarEntries(t, firstBytes)
	want := []string{
		"openbao-observability_0.1.0/",
		"openbao-observability_0.1.0/README.md",
		"openbao-observability_0.1.0/contracts/metrics.yaml",
		"openbao-observability_0.1.0/generated/grafana/overview.json",
	}
	if strings.Join(entries, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected archive entries:\n%s", strings.Join(entries, "\n"))
	}
}

func TestBundleRejectsUnsafeVersion(t *testing.T) {
	err := Bundle(BundleOptions{
		RepositoryRoot: t.TempDir(),
		Version:        "v0.1.0",
		Includes:       []string{"README.md"},
	})
	if err == nil {
		t.Fatal("expected version validation error")
	}
	if !strings.Contains(err.Error(), "without a leading v") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChecksumsWritesSortedSHA256File(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "z.txt"), "z\n", 0o644)
	mustWriteFile(t, filepath.Join(dir, "a.txt"), "a\n", 0o644)

	output := filepath.Join(dir, "checksums.txt")
	if err := Checksums(ChecksumOptions{Directory: dir, OutputPath: output}); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read checksums: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "  a.txt\n") || !strings.Contains(got, "  z.txt\n") {
		t.Fatalf("checksums do not reference expected files:\n%s", got)
	}
	if strings.Index(got, "a.txt") > strings.Index(got, "z.txt") {
		t.Fatalf("checksums are not sorted:\n%s", got)
	}
	if strings.Contains(got, "checksums.txt") {
		t.Fatalf("checksum file should not include itself:\n%s", got)
	}
}

func mustWriteFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func tarEntries(t *testing.T, data []byte) []string {
	t.Helper()
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open gzip archive: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var entries []string
	for {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("read tar archive: %v", err)
		}
		entries = append(entries, header.Name)
		if got, want := header.ModTime.Unix(), int64(1_700_000_000); got != want {
			t.Fatalf("entry %s mod time = %d, want %d", header.Name, got, want)
		}
	}
	return entries
}
