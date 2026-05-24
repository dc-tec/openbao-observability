package fixtures

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	baoapi "github.com/openbao/openbao/api/v2"
)

const (
	defaultScenarioAddress  = "http://127.0.0.1:18200"
	defaultScenarioUsername = "demo-admin"
	defaultScenarioPassword = "openbao-observability"
)

type ScenarioOptions struct {
	Address    string
	Token      string
	Username   string
	Password   string
	OutputPath string
}

type ScenarioReport struct {
	Address string         `json:"address"`
	Steps   []ScenarioStep `json:"steps"`
}

type ScenarioStep struct {
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type scenarioRunner struct {
	client *baoapi.Client
	report ScenarioReport
}

func RunScenario(ctx context.Context, opts ScenarioOptions) error {
	opts = opts.withDefaults()

	config := baoapi.DefaultConfig()
	config.Address = opts.Address
	client, err := baoapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("create OpenBao API client: %w", err)
	}

	runner := &scenarioRunner{
		client: client,
		report: ScenarioReport{Address: opts.Address},
	}

	if opts.Token != "" {
		client.SetToken(opts.Token)
		runner.addStep("use-provided-token", "", "success", "")
	} else if err := runner.loginUserpass(ctx, opts.Username, opts.Password, "login-userpass-admin"); err != nil {
		_ = runner.writeReport(opts.OutputPath)
		return err
	}

	if err := runner.run(ctx); err != nil {
		_ = runner.writeReport(opts.OutputPath)
		return err
	}

	if err := runner.writeReport(opts.OutputPath); err != nil {
		return err
	}
	fmt.Printf("fixture scenario completed against %s\n", opts.Address)
	return nil
}

func (o ScenarioOptions) withDefaults() ScenarioOptions {
	if o.Address == "" {
		o.Address = envString("BAO_ADDR", defaultScenarioAddress)
	}
	if o.Username == "" {
		o.Username = defaultScenarioUsername
	}
	if o.Password == "" {
		o.Password = defaultScenarioPassword
	}
	if o.OutputPath == "" {
		o.OutputPath = filepath.Join("fixtures", "captured", "openbao-2.5.4", "metadata", "openbao-2.5.4-compose-scenario.json")
	}
	return o
}

func (r *scenarioRunner) run(ctx context.Context) error {
	steps := []scenarioOperation{
		{name: "read-sys-auth", path: "sys/auth", operation: "read"},
		{name: "read-sys-mounts", path: "sys/mounts", operation: "read"},
		{name: "write-scenario-policy", path: "sys/policies/acl/scenario-app", operation: "write", data: map[string]any{
			"policy": `path "secret/data/apps/payments/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "secret/metadata/apps/payments/*" {
  capabilities = ["list", "read", "delete"]
}
`,
		}},
		{name: "write-userpass-scenario-user", path: "auth/userpass/users/scenario-user", operation: "write", data: map[string]any{
			"password":       defaultScenarioPassword,
			"token_policies": []string{"scenario-app"},
		}},
		{name: "write-kv", path: "secret/data/apps/payments/scenario", operation: "write", data: map[string]any{
			"data": map[string]any{
				"owner":    "payments",
				"scenario": "production-like-fixture",
				"tier":     "demo",
			},
		}},
		{name: "read-kv", path: "secret/data/apps/payments/scenario", operation: "read"},
		{name: "list-kv-metadata", path: "secret/metadata/apps/payments", operation: "list"},
		{name: "write-identity-entity", path: "identity/entity/name/scenario-service", operation: "write", data: map[string]any{
			"metadata": map[string]any{
				"environment": "local",
				"team":        "payments",
			},
			"policies": []string{"scenario-app"},
		}},
		{name: "write-identity-group", path: "identity/group/name/scenario-services", operation: "write", data: map[string]any{
			"metadata": map[string]any{
				"owner": "platform",
			},
			"policies": []string{"identity-auditor"},
			"type":     "internal",
		}},
	}

	for _, step := range steps {
		if err := r.runOperation(ctx, step); err != nil {
			return err
		}
	}

	if err := r.loginAppRole(ctx); err != nil {
		return err
	}
	if err := r.exerciseTokenLifecycle(ctx); err != nil {
		return err
	}
	if err := r.exerciseDatabaseLease(ctx); err != nil {
		return err
	}
	if err := r.exerciseTransit(ctx); err != nil {
		return err
	}
	if err := r.exerciseExpectedFailures(ctx); err != nil {
		return err
	}
	return nil
}

type scenarioOperation struct {
	name      string
	path      string
	operation string
	data      map[string]any
}

func (r *scenarioRunner) runOperation(ctx context.Context, step scenarioOperation) error {
	var err error
	switch step.operation {
	case "read":
		_, err = r.client.Logical().ReadWithContext(ctx, step.path)
	case "write":
		_, err = r.client.Logical().WriteWithContext(ctx, step.path, step.data)
	case "list":
		_, err = r.client.Logical().ListWithContext(ctx, step.path)
	default:
		return fmt.Errorf("unsupported scenario operation %q for %s", step.operation, step.name)
	}
	if err != nil {
		r.addStep(step.name, step.path, "error", err.Error())
		return fmt.Errorf("run scenario step %s at %s: %w", step.name, step.path, err)
	}
	r.addStep(step.name, step.path, "success", "")
	return nil
}

func (r *scenarioRunner) loginUserpass(ctx context.Context, username, password, stepName string) error {
	path := "auth/userpass/login/" + username
	secret, err := r.client.Logical().WriteWithContext(ctx, path, map[string]any{
		"password": password,
	})
	if err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("log in with userpass user %s: %w", username, err)
	}
	token, err := secret.TokenID()
	if err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("read userpass token for %s: %w", username, err)
	}
	if token == "" {
		err := errors.New("login response did not include a client token")
		r.addStep(stepName, path, "error", err.Error())
		return err
	}
	r.client.SetToken(token)
	r.addStep(stepName, path, "success", "")
	return nil
}

func (r *scenarioRunner) loginAppRole(ctx context.Context) error {
	roleIDSecret, err := r.client.Logical().ReadWithContext(ctx, "auth/approle/role/observability-app/role-id")
	if err != nil {
		r.addStep("read-approle-role-id", "auth/approle/role/observability-app/role-id", "error", err.Error())
		return fmt.Errorf("read AppRole role ID: %w", err)
	}
	r.addStep("read-approle-role-id", "auth/approle/role/observability-app/role-id", "success", "")

	secretIDSecret, err := r.client.Logical().WriteWithContext(ctx, "auth/approle/role/observability-app/secret-id", nil)
	if err != nil {
		r.addStep("write-approle-secret-id", "auth/approle/role/observability-app/secret-id", "error", err.Error())
		return fmt.Errorf("create AppRole secret ID: %w", err)
	}
	r.addStep("write-approle-secret-id", "auth/approle/role/observability-app/secret-id", "success", "")

	roleID, ok := roleIDSecret.Data["role_id"].(string)
	if !ok || roleID == "" {
		return errors.New("AppRole role-id response did not include data.role_id")
	}
	secretID, ok := secretIDSecret.Data["secret_id"].(string)
	if !ok || secretID == "" {
		return errors.New("AppRole secret-id response did not include data.secret_id")
	}

	if _, err := r.client.Logical().WriteWithContext(ctx, "auth/approle/login", map[string]any{
		"role_id":   roleID,
		"secret_id": secretID,
	}); err != nil {
		r.addStep("login-approle", "auth/approle/login", "error", err.Error())
		return fmt.Errorf("log in with AppRole: %w", err)
	}
	r.addStep("login-approle", "auth/approle/login", "success", "")
	return nil
}

func (r *scenarioRunner) exerciseTokenLifecycle(ctx context.Context) error {
	secret, err := r.client.Logical().WriteWithContext(ctx, "auth/token/create", map[string]any{
		"display_name": "scenario-token",
		"policies":     []string{"scenario-app"},
		"renewable":    true,
		"ttl":          "5m",
	})
	if err != nil {
		r.addStep("create-token", "auth/token/create", "error", err.Error())
		return fmt.Errorf("create scenario token: %w", err)
	}
	r.addStep("create-token", "auth/token/create", "success", "")

	token, err := secret.TokenID()
	if err != nil {
		return fmt.Errorf("read scenario token ID: %w", err)
	}
	if token == "" {
		return errors.New("token create response did not include a token")
	}

	tokenSteps := []scenarioOperation{
		{name: "lookup-token", path: "auth/token/lookup", operation: "write", data: map[string]any{"token": token}},
		{name: "renew-token", path: "auth/token/renew", operation: "write", data: map[string]any{"token": token, "increment": "60s"}},
		{name: "revoke-token", path: "auth/token/revoke", operation: "write", data: map[string]any{"token": token}},
	}
	for _, step := range tokenSteps {
		if err := r.runOperation(ctx, step); err != nil {
			return err
		}
	}
	return nil
}

func (r *scenarioRunner) exerciseDatabaseLease(ctx context.Context) error {
	secret, err := r.client.Logical().ReadWithContext(ctx, "database/creds/readonly")
	if err != nil {
		r.addStep("read-database-credentials", "database/creds/readonly", "error", err.Error())
		return fmt.Errorf("read dynamic database credentials: %w", err)
	}
	if secret == nil || secret.LeaseID == "" {
		r.addStep("read-database-credentials", "database/creds/readonly", "error", "missing lease_id")
		return errors.New("database credentials response did not include a lease_id")
	}
	r.addStep("read-database-credentials", "database/creds/readonly", "success", "lease_id_present")

	leaseSteps := []struct {
		name string
		run  func(context.Context, string) error
	}{
		{name: "lookup-database-lease", run: func(ctx context.Context, leaseID string) error {
			_, err := r.client.Sys().LookupWithContext(ctx, leaseID)
			return err
		}},
		{name: "renew-database-lease", run: func(ctx context.Context, leaseID string) error {
			_, err := r.client.Sys().RenewWithContext(ctx, leaseID, int((2 * time.Minute).Seconds()))
			return err
		}},
		{name: "revoke-database-lease", run: func(ctx context.Context, leaseID string) error {
			return r.client.Sys().RevokeWithContext(ctx, leaseID)
		}},
	}
	for _, step := range leaseSteps {
		if err := step.run(ctx, secret.LeaseID); err != nil {
			r.addStep(step.name, "sys/leases", "error", err.Error())
			return fmt.Errorf("%s: %w", step.name, err)
		}
		r.addStep(step.name, "sys/leases", "success", "")
	}
	return nil
}

func (r *scenarioRunner) exerciseTransit(ctx context.Context) error {
	if err := r.ensureTransitMount(ctx); err != nil {
		return err
	}

	key, err := r.client.Logical().ReadWithContext(ctx, "transit/keys/payments")
	if err != nil {
		r.addStep("read-transit-key", "transit/keys/payments", "error", err.Error())
		return fmt.Errorf("read transit key: %w", err)
	}
	if key == nil {
		r.addStep("read-transit-key", "transit/keys/payments", "error", "missing key")
		return errors.New("transit key payments does not exist")
	}
	r.addStep("read-transit-key", "transit/keys/payments", "success", "")

	plaintext := "openbao-observability-secret-engine-scenario"
	encrypted, err := r.client.Logical().WriteWithContext(ctx, "transit/encrypt/payments", map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString([]byte(plaintext)),
	})
	if err != nil {
		r.addStep("encrypt-transit", "transit/encrypt/payments", "error", err.Error())
		return fmt.Errorf("encrypt transit payload: %w", err)
	}
	if encrypted == nil || encrypted.Data == nil {
		err := errors.New("transit encrypt response did not include data")
		r.addStep("encrypt-transit", "transit/encrypt/payments", "error", err.Error())
		return err
	}
	ciphertext, ok := encrypted.Data["ciphertext"].(string)
	if !ok || ciphertext == "" {
		err := errors.New("transit encrypt response did not include data.ciphertext")
		r.addStep("encrypt-transit", "transit/encrypt/payments", "error", err.Error())
		return err
	}
	r.addStep("encrypt-transit", "transit/encrypt/payments", "success", "")

	decrypted, err := r.client.Logical().WriteWithContext(ctx, "transit/decrypt/payments", map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		r.addStep("decrypt-transit", "transit/decrypt/payments", "error", err.Error())
		return fmt.Errorf("decrypt transit payload: %w", err)
	}
	if decrypted == nil || decrypted.Data == nil {
		err := errors.New("transit decrypt response did not include data")
		r.addStep("decrypt-transit", "transit/decrypt/payments", "error", err.Error())
		return err
	}
	encodedPlaintext, ok := decrypted.Data["plaintext"].(string)
	if !ok || encodedPlaintext == "" {
		err := errors.New("transit decrypt response did not include data.plaintext")
		r.addStep("decrypt-transit", "transit/decrypt/payments", "error", err.Error())
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(encodedPlaintext)
	if err != nil {
		r.addStep("decrypt-transit", "transit/decrypt/payments", "error", err.Error())
		return fmt.Errorf("decode transit plaintext: %w", err)
	}
	if string(decoded) != plaintext {
		err := errors.New("transit decrypt plaintext did not match original payload")
		r.addStep("decrypt-transit", "transit/decrypt/payments", "error", err.Error())
		return err
	}
	r.addStep("decrypt-transit", "transit/decrypt/payments", "success", "")
	return nil
}

func (r *scenarioRunner) ensureTransitMount(ctx context.Context) error {
	mounts, err := r.client.Logical().ReadWithContext(ctx, "sys/mounts")
	if err != nil {
		r.addStep("ensure-transit-mount", "sys/mounts", "error", err.Error())
		return fmt.Errorf("read mounts before transit scenario: %w", err)
	}
	if mounts == nil || mounts.Data == nil {
		err := errors.New("sys/mounts response did not include data")
		r.addStep("ensure-transit-mount", "sys/mounts", "error", err.Error())
		return err
	}

	if _, ok := mounts.Data["transit/"]; !ok {
		if _, err := r.client.Logical().WriteWithContext(ctx, "sys/mounts/transit", map[string]any{
			"type":        "transit",
			"description": "Scenario Transit engine for observability reference captures.",
		}); err != nil {
			r.addStep("ensure-transit-mount", "sys/mounts/transit", "error", err.Error())
			return fmt.Errorf("enable transit mount: %w", err)
		}
		r.addStep("ensure-transit-mount", "sys/mounts/transit", "success", "")
	} else {
		r.addStep("ensure-transit-mount", "sys/mounts", "success", "")
	}

	key, err := r.client.Logical().ReadWithContext(ctx, "transit/keys/payments")
	if err != nil {
		r.addStep("ensure-transit-key", "transit/keys/payments", "error", err.Error())
		return fmt.Errorf("read transit key before scenario: %w", err)
	}
	if key != nil {
		r.addStep("ensure-transit-key", "transit/keys/payments", "success", "")
		return nil
	}

	if _, err := r.client.Logical().WriteWithContext(ctx, "transit/keys/payments", map[string]any{
		"type":                   "aes256-gcm96",
		"derived":                false,
		"exportable":             false,
		"allow_plaintext_backup": false,
	}); err != nil {
		r.addStep("ensure-transit-key", "transit/keys/payments", "error", err.Error())
		return fmt.Errorf("create transit key: %w", err)
	}
	r.addStep("ensure-transit-key", "transit/keys/payments", "success", "")
	return nil
}

func (r *scenarioRunner) exerciseExpectedFailures(ctx context.Context) error {
	if _, err := r.client.Logical().WriteWithContext(ctx, "auth/userpass/login/demo-reader", map[string]any{
		"password": "incorrect-password",
	}); err != nil {
		r.addStep("failed-userpass-login", "auth/userpass/login/demo-reader", "expected_error", "")
	} else {
		return errors.New("failed-userpass-login unexpectedly succeeded")
	}

	config := baoapi.DefaultConfig()
	config.Address = r.client.Address()
	readerClient, err := baoapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("create reader client: %w", err)
	}
	readerRunner := &scenarioRunner{client: readerClient}
	if err := readerRunner.loginUserpass(ctx, "demo-reader", defaultScenarioPassword, "login-userpass-reader"); err != nil {
		return err
	}
	readerToken := readerClient.Token()
	r.addStep("login-userpass-reader", "auth/userpass/login/demo-reader", "success", "")

	readerClient.SetToken(readerToken)
	if _, err := readerClient.Logical().WriteWithContext(ctx, "secret/data/apps/payments/denied", map[string]any{
		"data": map[string]any{"denied": true},
	}); err != nil {
		r.addStep("denied-reader-kv-write", "secret/data/apps/payments/denied", "expected_error", "")
		return nil
	}
	return errors.New("denied-reader-kv-write unexpectedly succeeded")
}

func (r *scenarioRunner) addStep(name, path, status, detail string) {
	r.report.Steps = append(r.report.Steps, ScenarioStep{
		Name:   name,
		Path:   path,
		Status: status,
		Detail: detail,
	})
}

func (r *scenarioRunner) writeReport(path string) error {
	if path == "" {
		return nil
	}
	content, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scenario report: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create scenario report directory: %w", err)
	}
	return writeFile(path, content)
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
