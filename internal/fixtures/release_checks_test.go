package fixtures

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReleaseAudit(t *testing.T) {
	const entries = `{"type":"response","request":{"path":"identity/entity/name/release-malformed"},` +
		`"response":{"data":{"error":"hmac-sha256:fixture"}}}
{"type":"response","request":{"path":"sys/policies/acl/release-audit"}}
{"type":"response","request":{"path":"demo/data/release-denied"},"error":"permission denied"}
{"type":"request","request":{"path":"demo/data/release-template"},"error":"Forbidden"}
`
	for _, test := range []struct {
		name, data, wantError string
	}{
		{"complete", entries, ""},
		{"plaintext marker", entries + `{"error":"` + auditMarker + `"}`, "malformed value leaked"},
		{"unprotected error", strings.ReplaceAll(entries, "hmac-sha256:fixture", "unprotected"), "not HMAC-protected"},
		{
			"missing template denial", strings.ReplaceAll(entries, "Forbidden", "invalid request"),
			"missing expected release audit entry",
		},
		{
			"uncanonical policy path",
			strings.ReplaceAll(entries, "sys/policies/acl/release-audit", "sys/policies/acl/RELEASE-AUDIT"),
			"missing expected release audit entry",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "audit.jsonl")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			err := checkReleaseAudit(path)
			if test.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}
