package splunk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveClientRequiresSecureEndpointAndToken(t *testing.T) {
	for _, endpoint := range []string{
		"", "http://example.com:8089", "https://user:pass@example.com",
		"https://example.com/path", "https://example.com?token=secret",
	} {
		if _, _, err := liveClient(LiveOptions{URL: endpoint, Token: "test"}); err == nil {
			t.Fatalf("accepted %q", endpoint)
		}
	}
	if _, _, err := liveClient(LiveOptions{URL: "https://example.com"}); err == nil {
		t.Fatal("accepted missing token")
	}
}

func TestExecuteSearchRejectsHTTPAndSearchFailures(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{"valid", `{"results":[{"audit_events":"8"}]}`, 200, false},
		{"unauthorized", `private server data`, 401, true},
		{"parse error", `{"messages":[{"type":"ERROR","text":"private"}],"results":[{}]}`, 200, true},
		{"warning", `{"messages":[{"type":"WARN"}],"results":[{}]}`, 200, true},
		{"empty", `{"results":[]}`, 200, true},
		{"malformed", `<html>`, 200, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("invalid API request")
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("exec_mode") != "oneshot" || !strings.HasPrefix(r.Form.Get("search"), "| makeresults") {
					t.Error("expected synthetic oneshot search")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			rows, err := executeSearch(context.Background(), server.Client(), server.URL, "Bearer test-token", fixtureSearch())
			if (err != nil) != tc.wantError {
				t.Fatalf("rows=%v, error=%v", rows, err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("leaked response body")
			}
		})
	}
}

func TestFixturePreservesLateFieldsAndSecretSentinels(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(syntheticAudit), "\n")
	if len(lines) != 8 || strings.Index(lines[7], `"id"`) < 5000 {
		t.Fatal("missing late-field fixture")
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatal("invalid audit fixture")
		}
	}
	query := fixtureSearch()
	if strings.Contains(query, "index=") || !strings.Contains(query, `host="fixture-node"`) {
		t.Fatal("unsafe fixture source")
	}
}

func TestNormalizedRowsRejectSecretPayloads(t *testing.T) {
	rows := make([]map[string]json.RawMessage, 8)
	for i := range rows {
		rows[i] = map[string]json.RawMessage{
			"request_id": json.RawMessage(`"late-1"`), "caller": json.RawMessage(`"(missing)"`),
			"logical_namespace": json.RawMessage(`"(root)"`),
		}
	}
	if err := verifyNormalizedRows(rows); err != nil {
		t.Fatal(err)
	}
	rows[0]["_raw"] = json.RawMessage(`"private"`)
	if err := verifyNormalizedRows(rows); err == nil {
		t.Fatal("accepted _raw")
	}
	delete(rows[0], "_raw")
	rows[0]["caller"] = json.RawMessage(`"FIXTURE_TOKEN_DO_NOT_OUTPUT"`)
	if err := verifyNormalizedRows(rows); err == nil {
		t.Fatal("accepted secret sentinel")
	}
}

func TestInsecureTLSIsLimitedToLoopback(t *testing.T) {
	for _, endpoint := range []string{"https://example.com:8089", "https://127.0.0.1.example.com:8089"} {
		if _, _, err := liveClient(LiveOptions{URL: endpoint, Token: "test", InsecureLoopback: true}); err == nil {
			t.Fatalf("accepted insecure remote URL %s", endpoint)
		}
	}
	client, _, err := liveClient(LiveOptions{
		URL: "https://127.0.0.1:8089", Token: "test", AuthScheme: "Splunk", InsecureLoopback: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.CloseIdleConnections()
	opts := LiveOptions{URL: "https://127.0.0.1:8089", Token: "test", AuthScheme: "Basic"}
	if _, _, err := liveClient(opts); err == nil {
		t.Fatal("accepted an unsupported authentication scheme")
	}
}
