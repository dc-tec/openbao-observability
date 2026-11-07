package splunk

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

//go:embed testdata/audit.ndjson
var syntheticAudit string

// LiveOptions identifies an authorized test instance. No events are indexed or apps installed.
type LiveOptions struct {
	Options
	URL              string
	Token            string
	CACertPath       string
	AuthScheme       string
	InsecureLoopback bool
}

type searchResponse struct {
	Results  []map[string]json.RawMessage `json:"results"`
	Messages []searchMessage              `json:"messages"`
}
type searchMessage struct {
	Type string `json:"type"`
}

// ValidateQueries executes native SPL with synthetic events instead of an audit index.
// It does not qualify index ingestion, installed macros, token substitution, or UI rendering.
func ValidateQueries(ctx context.Context, opts LiveOptions) error {
	profile, err := contracts.LoadSplunkProfile(opts.ContractPath, opts.RepositoryRoot)
	if err != nil {
		return err
	}
	client, endpoint, err := liveClient(opts)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	source := fixtureSearch() + normalization(*profile)
	// Test the normalization before any panel's table or stats can hide leaked fields.
	rows, err := executeSearch(ctx, client, endpoint, authorization(opts), source)
	if err != nil {
		return fmt.Errorf("normalize synthetic audit: %w", err)
	}
	if err := verifyNormalizedRows(rows); err != nil {
		return err
	}
	for _, binding := range profile.Dashboards {
		for _, panel := range binding.Panels {
			if !hasAuditExpectation(panel.SourcePanel) || binding.ID != "openbao_audit_investigation" {
				continue
			}
			query := strings.Replace(panel.Search, auditMacro, source, 1)
			rows, err := executeSearch(ctx, client, endpoint, authorization(opts), query)
			if err != nil {
				return fmt.Errorf("panel %s: %w", panel.SourcePanel, err)
			}
			if err := verifyPanelRows(panel.SourcePanel, rows); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasAuditExpectation(id string) bool {
	return containsString([]string{
		"filtered-audit-events", "risky-system-mutations", "auth-activity", "audit-response-errors",
		"filtered-audit-events-by-node", "risky-system-activity-by-node", "request-id-drilldown",
		"auth-activity-events", "audit-error-events",
	}, id)
}

func liveClient(opts LiveOptions) (*http.Client, string, error) {
	endpoint, err := liveEndpoint(opts.URL)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(opts.Token) == "" {
		return nil, "", fmt.Errorf("set SPLUNK_TOKEN to a test-instance bearer token")
	}
	scheme := opts.AuthScheme
	if scheme != "" && scheme != "Bearer" && scheme != "Splunk" {
		return nil, "", fmt.Errorf("unsupported Splunk authentication scheme")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.InsecureLoopback {
		if !isLoopback(endpoint) {
			return nil, "", fmt.Errorf("TLS verification bypass requires a literal loopback host")
		}
		// #nosec G402 -- This opt-in is restricted to the disposable loopback trial API.
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	}
	if opts.CACertPath != "" {
		pem, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, "", err
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, "", err
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, "", fmt.Errorf("CA file contains no certificates")
		}
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	}
	client := &http.Client{
		Transport: transport, Timeout: 45 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("the Splunk API redirects are disabled")
		},
	}
	endpoint.Path = "/services/search/jobs"
	return client, endpoint.String(), nil
}

func executeSearch(
	ctx context.Context, client *http.Client, endpoint, auth, query string,
) ([]map[string]json.RawMessage, error) {
	values := url.Values{
		"search": {query}, "exec_mode": {"oneshot"}, "output_mode": {"json"},
		"count": {"100"}, "earliest_time": {"-1h"}, "latest_time": {"now"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", auth)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("the Splunk search returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var result searchResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode Splunk search response: %w", err)
	}
	for _, message := range result.Messages {
		if message.Type == "ERROR" || message.Type == "FATAL" || message.Type == "WARN" {
			return nil, fmt.Errorf("the Splunk search returned a %s message; review the test instance", message.Type)
		}
	}
	if len(result.Results) == 0 {
		return nil, fmt.Errorf("the Splunk search returned no synthetic results")
	}
	return result.Results, nil
}

func fixtureSearch() string {
	lines := strings.Split(strings.TrimSpace(syntheticAudit), "\n")
	var query strings.Builder
	fmt.Fprintf(&query, "| makeresults count=%d | streamstats count AS fixture_row | eval _raw=case(", len(lines))
	for i, line := range lines {
		if i > 0 {
			query.WriteString(",")
		}
		fmt.Fprintf(&query, "fixture_row=%d,%s", i+1, strconv.Quote(line))
	}
	query.WriteString(`), host="fixture-node", _time=now()-60`)
	return query.String()
}

func verifyNormalizedRows(rows []map[string]json.RawMessage) error {
	if len(rows) != 8 {
		return fmt.Errorf("normalized event count = %d, want 8", len(rows))
	}
	lateID, missingCaller, rootNamespace := false, false, false
	allowed := append(contracts.SplunkAuditFields(), "_time", "host")
	for _, row := range rows {
		for name, value := range row {
			// Splunk may include internal search metadata; _raw must never survive projection.
			if name == "_raw" || (!strings.HasPrefix(name, "_") && !containsString(allowed, name)) ||
				strings.Contains(string(value), "FIXTURE_SECRET") || strings.Contains(string(value), "FIXTURE_TOKEN") {
				return fmt.Errorf("unexpected normalized field %s", name)
			}
		}
		lateID = lateID || string(row["request_id"]) == `"late-1"`
		missingCaller = missingCaller || string(row["caller"]) == `"(missing)"`
		rootNamespace = rootNamespace || string(row["logical_namespace"]) == `"(root)"`
	}
	if !lateID || !missingCaller || !rootNamespace {
		return fmt.Errorf("normalization lost late fields or missing-field defaults")
	}
	return nil
}

func verifyPanelRows(id string, rows []map[string]json.RawMessage) error {
	scalar := map[string]struct {
		Field string
		Count int
	}{
		"filtered-audit-events": {"audit_events", 8}, "risky-system-mutations": {"audit_events", 2},
		"auth-activity": {"audit_events", 3}, "audit-response-errors": {"response_errors", 1},
	}
	if expected, ok := scalar[id]; ok {
		if len(rows) != 1 {
			return fmt.Errorf("panel %s returned %d rows, want 1", id, len(rows))
		}
		return checkCount(id, rows[0][expected.Field], expected.Count)
	}
	switch id {
	case "filtered-audit-events-by-node", "risky-system-activity-by-node":
		total := 0
		for _, row := range rows {
			value, err := rawCount(row["fixture-node"])
			if err != nil {
				return err
			}
			total += value
		}
		expected := 8
		if id == "risky-system-activity-by-node" {
			expected = 2
		}
		if total != expected {
			return fmt.Errorf("panel %s total = %d, want %d", id, total, expected)
		}
	case "request-id-drilldown", "auth-activity-events", "audit-error-events":
		expected := map[string]int{"request-id-drilldown": 8, "auth-activity-events": 3, "audit-error-events": 1}[id]
		if len(rows) != expected {
			return fmt.Errorf("panel %s rows = %d, want %d", id, len(rows), expected)
		}
	default:
		return fmt.Errorf("no synthetic expectation for panel %s", id)
	}
	return nil
}

func checkCount(id string, value json.RawMessage, expected int) error {
	actual, err := rawCount(value)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("panel %s count = %d, want %d", id, actual, expected)
	}
	return nil
}

func rawCount(value json.RawMessage) (int, error) {
	return strconv.Atoi(strings.Trim(string(value), `"`))
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func liveEndpoint(rawURL string) (*url.URL, error) {
	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Splunk URL")
	}
	if endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" ||
		(endpoint.Path != "" && endpoint.Path != "/") {
		return nil, fmt.Errorf("the Splunk URL must contain only scheme, host, and port")
	}
	local := isLoopback(endpoint)
	if endpoint.Scheme != "https" && (!local || endpoint.Scheme != "http") {
		return nil, fmt.Errorf("the Splunk management API requires HTTPS except on loopback")
	}

	return endpoint, nil
}

func isLoopback(endpoint *url.URL) bool {
	return endpoint.Hostname() == "localhost" || endpoint.Hostname() == "127.0.0.1" || endpoint.Hostname() == "::1"
}

func authorization(opts LiveOptions) string {
	scheme := opts.AuthScheme
	if scheme == "" {
		scheme = "Bearer"
	}
	return scheme + " " + opts.Token
}
