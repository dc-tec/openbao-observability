package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplunkAppIsInstallableVersionedAndDeterministic(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "generated/splunk/app/openbao_observability")
	conf := "[launcher]\nversion = 0.0.0-dev\n[id]\nname = openbao_observability\nversion = 0.0.0-dev\n" +
		"[package]\nid = openbao_observability\n"
	mustWriteFile(t, filepath.Join(app, "default/app.conf"), conf)
	mustWriteFile(t, filepath.Join(app, "default/macros.conf"), "[source]\n")
	mustWriteFile(t, filepath.Join(app, "default/data/ui/views/overview.xml"), "<dashboard/>\n")
	mustWriteFile(t, filepath.Join(app, "metadata/default.meta"), "[]\naccess = read : [ admin ]\n")
	mustWriteFile(t, filepath.Join(app, "local/macros.conf"), "private install override")
	mustWriteFile(t, filepath.Join(root, "LICENSE"), "license fixture")
	opts := SplunkAppOptions{RepositoryRoot: root, Version: "1.2.3", SourceDateEpoch: 1_700_000_000}
	if err := SplunkApp(opts); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, "dist/release/openbao_observability_1.2.3.spl"))
	if err != nil {
		t.Fatal(err)
	}
	opts.OutputPath = "second.spl"
	if err := SplunkApp(opts); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, opts.OutputPath))
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("app package is not deterministic", err)
	}
	source, err := os.ReadFile(filepath.Join(app, "default/app.conf"))
	if err != nil || string(source) != conf {
		t.Fatal("packaging changed generated input", err)
	}
	entries := readSplunkApp(t, first)
	if len(entries) != 6 || strings.Count(entries["openbao_observability/default/app.conf"], "version = 1.2.3") != 2 {
		t.Fatal("app package has unexpected contents or version", entries)
	}
}

func TestSplunkAppRejectsMissingVersion(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "generated/splunk/app/openbao_observability/default/app.conf"), "[launcher]\n")
	mustWriteFile(t, filepath.Join(root, "generated/splunk/app/openbao_observability/metadata/default.meta"), "[]\n")
	if err := SplunkApp(SplunkAppOptions{RepositoryRoot: root}); err == nil {
		t.Fatal("accepted an app without a version")
	}
}

func readSplunkApp(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	archive := tar.NewReader(reader)
	entries := map[string]string{}
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(header.Name, "openbao_observability/") {
			t.Fatal("app has an incorrect archive root", header.Name)
		}
		data, err := io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = string(data)
	}
	return entries
}
