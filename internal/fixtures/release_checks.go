package fixtures

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	baoapi "github.com/openbao/openbao/api/v2"
)

const auditMarker = "observability-malformed-value-canary"

type fixtureResponse struct {
	code   int
	header http.Header
	body   []byte
}

// requestFixture does not retry: consistency fixtures must observe individual 429 responses.
func requestFixture(ctx context.Context, port int, token, method, path, body string,
	headers map[string]string,
) (fixtureResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method,
		fmt.Sprintf("http://127.0.0.1:%d/v1/%s", port, path), strings.NewReader(body))
	if err != nil {
		return fixtureResponse{}, err
	}
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fixtureResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	return fixtureResponse{resp.StatusCode, resp.Header, data}, err
}

func expectFixtureStatus(resp fixtureResponse, err error, status int) error {
	if err != nil {
		return err
	}
	if resp.code != status {
		var detail struct {
			Errors []string `json:"errors"`
		}
		_ = json.Unmarshal(resp.body, &detail)
		return fmt.Errorf("fixture HTTP status = %d, want %d: %s", resp.code, status, strings.Join(detail.Errors, "; "))
	}
	return nil
}

func (r *captureRun) exerciseReleaseAudit(ctx context.Context, port int) error {
	token := r.options.RootToken
	resp, err := requestFixture(ctx, port, token, http.MethodPost, "identity/entity/name/release-malformed",
		`{"metadata":["`+auditMarker+`"]}`, nil)
	if err := expectFixtureStatus(resp, err, http.StatusBadRequest); err != nil {
		return err
	}
	if strings.Contains(string(resp.body), auditMarker) {
		return fmt.Errorf("malformed value leaked in API error")
	}
	client, err := fixtureClient(port, token)
	if err != nil {
		return err
	}
	if err := client.Sys().PutPolicyWithContext(ctx, "RELEASE-AUDIT",
		`path "demo/data/allowed" { capabilities = ["read"] }`); err != nil {
		return err
	}
	secret, err := client.Auth().Token().CreateWithContext(ctx,
		&baoapi.TokenCreateRequest{Policies: []string{"release-audit"}, NoDefaultPolicy: true, TTL: "5m"})
	if err != nil {
		return err
	}
	resp, err = requestFixture(ctx, port, secret.Auth.ClientToken, http.MethodGet, "demo/data/release-denied", "", nil)
	if err := expectFixtureStatus(resp, err, http.StatusForbidden); err != nil {
		return err
	}
	return exerciseTemplateDenial(ctx, port, client)
}

func exerciseTemplateDenial(ctx context.Context, port int, client *baoapi.Client) error {
	if err := client.Sys().PutPolicyWithContext(ctx, "release-template",
		`path "demo/data/{{identity.entity.metadata.key}}/*" { capabilities = ["read"] }`); err != nil {
		return err
	}
	resp, err := requestFixture(ctx, port, client.Token(), http.MethodPost, "auth/userpass/users/release-template",
		`{"password":"fixture-only-password","token_policies":["release-template"]}`, nil)
	if err := expectFixtureStatus(resp, err, http.StatusNoContent); err != nil {
		return err
	}
	resp, err = requestFixture(ctx, port, "", http.MethodPost, "auth/userpass/login/release-template",
		`{"password":"fixture-only-password"}`, nil)
	if err := expectFixtureStatus(resp, err, http.StatusOK); err != nil {
		return err
	}
	var login baoapi.Secret
	if err := json.Unmarshal(resp.body, &login); err != nil {
		return err
	}
	if login.Auth == nil || login.Auth.EntityID == "" {
		return fmt.Errorf("template fixture has no entity")
	}
	resp, err = requestFixture(ctx, port, client.Token(), http.MethodPost, "identity/entity/id/"+login.Auth.EntityID,
		`{"metadata":{"key":"fixture/+"}}`, nil)
	if err := expectFixtureStatus(resp, err, http.StatusNoContent); err != nil {
		return err
	}
	resp, err = requestFixture(ctx, port, login.Auth.ClientToken, http.MethodGet, "demo/data/release-template", "", nil)
	// The templating rejection is returned as HTTP 400 with a Forbidden error.
	return expectFixtureStatus(resp, err, http.StatusBadRequest)
}

func checkReleaseAudit(path string) error {
	file, err := os.Open(path) // #nosec G304 -- The local validation command selects fixture paths.
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	want := map[string]struct{ entryType, errorFragment string }{
		"identity/entity/name/release-malformed": {"response", ""},
		"sys/policies/acl/release-audit":         {"response", ""},
		"demo/data/release-denied":               {"response", "permission denied"},
		"demo/data/release-template":             {"request", "forbidden"},
	}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), auditMarker) {
			return fmt.Errorf("malformed value leaked in audit fixture %s", path)
		}
		var entry struct {
			Type    string `json:"type"`
			Error   string `json:"error"`
			Request struct {
				Path string `json:"path"`
			} `json:"request"`
			Response struct {
				Data struct {
					Error string `json:"error"`
				} `json:"data"`
			} `json:"response"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return err
		}
		expected, ok := want[entry.Request.Path]
		if ok && entry.Type == expected.entryType && strings.Contains(strings.ToLower(entry.Error), expected.errorFragment) {
			if entry.Request.Path == "identity/entity/name/release-malformed" &&
				!strings.HasPrefix(entry.Response.Data.Error, "hmac-sha256:") {
				return fmt.Errorf("malformed response error is not HMAC-protected in %s", path)
			}
			seen[entry.Request.Path] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for path := range want {
		if !seen[path] {
			return fmt.Errorf("missing expected release audit entry for %s", path)
		}
	}
	return nil
}

func (r *captureRun) captureConsistency(ctx context.Context, leader, standby raftNode, prefix string) error {
	if strings.HasPrefix(r.options.Version, "2.6.") {
		return nil
	}
	const path = "secret/data/observability/consistency"
	resp, err := requestFixture(ctx, leader.Port, fixtureAdminToken, http.MethodPost, path,
		`{"data":{"sample":"consistency-fixture"}}`, nil)
	if err := expectFixtureStatus(resp, err, http.StatusOK); err != nil {
		return err
	}
	index, err := baoapi.DecodeIndexValue(resp.header.Get("X-Vault-Index"))
	if err != nil {
		return err
	}
	if index == nil || index.Value == "" {
		return fmt.Errorf("raft write did not return X-Vault-Index")
	}
	actual, err := index.Encode()
	if err != nil {
		return err
	}
	if err := awaitFixture(ctx, func() (bool, error) {
		result, err := requestFixture(ctx, standby.Port, fixtureAdminToken, http.MethodGet, path, "",
			map[string]string{"X-Vault-Index": actual, "X-Vault-Inconsistent": "fail"})
		return result.code == http.StatusOK && strings.Contains(string(result.body), "consistency-fixture"), err
	}); err != nil {
		return fmt.Errorf("standby read-after-write: %w", err)
	}
	value, err := strconv.ParseUint(index.Value, 10, 64)
	if err != nil {
		return err
	}
	// A future index makes the stale-read condition deterministic without pausing replication.
	index.Value = strconv.FormatUint(value+1000000, 10)
	future, err := index.Encode()
	if err != nil {
		return err
	}
	if err := checkConsistencyFallbacks(ctx, standby.Port, path, future); err != nil {
		return err
	}
	return writeFile(raftClusterMetadataPath(r.options, prefix, "consistency.txt"),
		[]byte("read-after-write=200\nfail=429\nawait-state=429\nforward-active-node=200\n"))
}

func checkConsistencyFallbacks(ctx context.Context, port int, path, future string) error {
	for _, behavior := range []string{"fail", "await-state", "forward-active-node"} {
		resp, err := requestFixture(ctx, port, fixtureAdminToken, http.MethodGet, path, "",
			map[string]string{"X-Vault-Index": future, "X-Vault-Inconsistent": behavior})
		want := http.StatusTooManyRequests
		if behavior == "forward-active-node" {
			want = http.StatusOK
		}
		if err := expectFixtureStatus(resp, err, want); err != nil {
			return fmt.Errorf("consistency %s: %w", behavior, err)
		}
		if want == http.StatusTooManyRequests && resp.header.Get("Retry-After") == "" {
			return fmt.Errorf("consistency %s has no Retry-After", behavior)
		}
	}
	return nil
}
