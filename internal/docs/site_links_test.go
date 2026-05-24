package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifySiteLinksAcceptsBasePathLinks(t *testing.T) {
	root := t.TempDir()
	writeSiteFile(t, root, "index.html", `<a href=/openbao-observability/concepts/>Concepts</a>`)
	writeSiteFile(t, root, "concepts/index.html", `<a href=/openbao-observability/>Home</a>`)

	err := VerifySiteLinks(SiteLinkOptions{
		SiteRoot: root,
		BaseURL:  "https://dc-tec.github.io/openbao-observability/",
	})
	if err != nil {
		t.Fatalf("verify site links: %v", err)
	}
}

func TestVerifySiteLinksRejectsRootAbsoluteLinksWithoutBasePath(t *testing.T) {
	root := t.TempDir()
	writeSiteFile(t, root, "index.html", `<a href=/concepts/audit-logs-as-security-records/>Audit logs</a>`)
	writeSiteFile(t, root, "concepts/audit-logs-as-security-records/index.html", `ok`)

	err := VerifySiteLinks(SiteLinkOptions{
		SiteRoot: root,
		BaseURL:  "https://dc-tec.github.io/openbao-observability/",
	})
	if err == nil {
		t.Fatal("expected base-path validation error")
	}
	if !strings.Contains(err.Error(), "does not include base path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifySiteLinksRejectsMissingTargets(t *testing.T) {
	root := t.TempDir()
	writeSiteFile(t, root, "index.html", `<a href=/openbao-observability/missing/>Missing</a>`)

	err := VerifySiteLinks(SiteLinkOptions{
		SiteRoot: root,
		BaseURL:  "https://dc-tec.github.io/openbao-observability/",
	})
	if err == nil {
		t.Fatal("expected missing target validation error")
	}
	if !strings.Contains(err.Error(), "target does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeSiteFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create site directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write site file: %v", err)
	}
}
