package fixtures

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	baoapi "github.com/openbao/openbao/api/v2"
)

const (
	defaultScenarioAddress  = "http://127.0.0.1:18200"
	defaultScenarioUsername = "demo-admin"
	defaultScenarioPassword = "openbao-observability"
	defaultPostgresHost     = "postgres"
	auditCanaryToken        = "openbao-observability-audit-canary-token"
	auditCanaryPath         = "secret/data/observability/audit-canary"
	fixtureNamespace        = "team-a"
	fixtureNestedNamespace  = fixtureNamespace + "/payments"
	nsKVDesc                = "Namespace KV engine for observability reference captures."
	nsUserpassDesc          = "Namespace userpass auth method for observability reference captures."
	nsAppRoleDesc           = "Namespace AppRole auth method for observability reference captures."
	nsDBDesc                = "Namespace PostgreSQL dynamic secrets engine for observability reference captures."
	nsTransitDesc           = "Namespace Transit engine for observability reference captures."
	nsPKIDesc               = "Namespace PKI engine for observability reference captures."
	nestedKVDesc            = "Nested namespace KV engine for observability reference captures."
	scenarioKVDesc          = "Scenario KV v1 engine for observability reference captures."
	scenarioPKIDesc         = "Scenario PKI engine for observability reference captures."
	scenarioTransitDesc     = "Scenario Transit engine for observability reference captures."
)

var (
	postgresCreationStatements = []string{
		"CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';",
		"GRANT CONNECT ON DATABASE openbao_app TO \"{{name}}\";",
	}
	postgresRevocationStatements = []string{}
	postgresInvalidStatement     = []string{
		"SELECT openbao_observability_fixture_missing_function();",
	}
)

type ScenarioOptions struct {
	Address      string
	Token        string
	Username     string
	Password     string
	PostgresHost string
	OutputPath   string
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
	client       *baoapi.Client
	postgresHost string
	report       ScenarioReport
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
		client:       client,
		postgresHost: opts.PostgresHost,
		report:       ScenarioReport{Address: opts.Address},
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
	if o.PostgresHost == "" {
		o.PostgresHost = envString("POSTGRES_HOST", defaultPostgresHost)
	}
	if o.OutputPath == "" {
		o.OutputPath = filepath.Join(
			"fixtures",
			"captured",
			"openbao-2.5.4",
			"metadata",
			"openbao-2.5.4-compose-scenario.json",
		)
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
		{
			name:      "write-userpass-scenario-user",
			path:      "auth/userpass/users/scenario-user",
			operation: "write",
			data: map[string]any{
				"password":       defaultScenarioPassword,
				"token_policies": []string{"scenario-app"},
			},
		},
		{name: "write-kv", path: "secret/data/apps/payments/scenario", operation: "write", data: map[string]any{
			"data": map[string]any{
				"owner":    "payments",
				"scenario": "production-like-fixture",
				"tier":     "demo",
			},
		}},
		{name: "read-kv", path: "secret/data/apps/payments/scenario", operation: "read"},
		{name: "list-kv-metadata", path: "secret/metadata/apps/payments", operation: "list"},
		{
			name:      "write-identity-entity",
			path:      "identity/entity/name/scenario-service",
			operation: "write",
			data: map[string]any{
				"metadata": map[string]any{
					"environment": "local",
					"team":        "payments",
				},
				"policies": []string{"scenario-app"},
			},
		},
		{
			name:      "write-identity-group",
			path:      "identity/group/name/scenario-services",
			operation: "write",
			data: map[string]any{
				"metadata": map[string]any{
					"owner": "platform",
				},
				"policies": []string{"identity-auditor"},
				"type":     "internal",
			},
		},
	}

	for _, step := range steps {
		if err := r.runOperation(ctx, step); err != nil {
			return err
		}
	}

	if err := r.ensureAuditCanary(ctx); err != nil {
		return err
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
	if err := r.exerciseKVv1(ctx); err != nil {
		return err
	}
	if err := r.exerciseTransit(ctx); err != nil {
		return err
	}
	if err := r.exercisePKI(ctx); err != nil {
		return err
	}
	if err := r.exerciseNamespace(ctx); err != nil {
		return err
	}
	if err := r.exerciseFeatureExpectedFailures(ctx); err != nil {
		return err
	}
	if err := r.exerciseExpectedFailures(ctx); err != nil {
		return err
	}
	return nil
}

func (r *scenarioRunner) exerciseNamespace(ctx context.Context) error {
	if err := r.ensureNamespace(ctx, fixtureNamespace); err != nil {
		return err
	}
	if err := r.runOperation(ctx, scenarioOperation{
		name:      "read-namespace-team-a",
		path:      "sys/namespaces/" + fixtureNamespace,
		operation: "read",
	}); err != nil {
		return err
	}
	if err := r.runOperation(ctx, scenarioOperation{
		name:      "list-namespaces",
		path:      "sys/namespaces",
		operation: "list",
	}); err != nil {
		return err
	}

	namespaceClient := r.client.WithNamespace(fixtureNamespace)
	return r.withClient(namespaceClient, func() error {
		if err := r.ensureMount(
			ctx,
			"secret",
			"kv",
			nsKVDesc,
			map[string]any{"version": "2"},
		); err != nil {
			return err
		}
		r.renameLastStep("ensure-secret-mount", "ensure-namespace-secret-mount")

		steps := []scenarioOperation{
			{
				name:      "write-namespace-policy",
				path:      "sys/policies/acl/namespace-app",
				operation: "write",
				data: map[string]any{
					"policy": `path "secret/data/apps/team-a/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "secret/metadata/apps/team-a/*" {
  capabilities = ["list", "read", "delete"]
}
`,
				},
			},
			{
				name:      "write-namespace-user",
				path:      "auth/userpass/users/namespace-user",
				operation: "write",
				data: map[string]any{
					"password":       defaultScenarioPassword,
					"token_policies": []string{"namespace-app"},
				},
			},
			{
				name:      "write-namespace-kv",
				path:      "secret/data/apps/team-a/scenario",
				operation: "write",
				data: map[string]any{
					"data": map[string]any{
						"owner":    "team-a",
						"scenario": "namespace-fixture",
						"tier":     "demo",
					},
				},
			},
			{name: "read-namespace-kv", path: "secret/data/apps/team-a/scenario", operation: "read"},
			{name: "list-namespace-kv-metadata", path: "secret/metadata/apps/team-a", operation: "list"},
		}
		if err := r.ensureAuth(
			ctx,
			"userpass",
			"userpass",
			nsUserpassDesc,
			"ensure-namespace-userpass-auth",
		); err != nil {
			return err
		}
		for _, step := range steps {
			if err := r.runOperation(ctx, step); err != nil {
				return err
			}
		}

		if err := r.exerciseNamespaceUserpass(ctx); err != nil {
			return err
		}
		if err := r.exerciseNamespaceToken(ctx); err != nil {
			return err
		}
		if err := r.exerciseNamespaceAppRole(ctx); err != nil {
			return err
		}
		if err := r.exerciseNamespaceTransit(ctx); err != nil {
			return err
		}
		if err := r.exerciseNamespacePKI(ctx); err != nil {
			return err
		}
		if err := r.exerciseNamespaceDatabaseLease(ctx); err != nil {
			return err
		}
		if err := r.exerciseNestedNamespace(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (r *scenarioRunner) ensureNamespace(ctx context.Context, namespacePath string) error {
	path := "sys/namespaces/" + namespacePath
	stepName := "ensure-namespace-" + namespaceStepSuffix(namespacePath)
	if existing, err := r.client.Logical().ReadWithContext(ctx, path); err == nil && existing != nil {
		r.addStep(stepName, path, "success", "")
		return nil
	}

	_, err := r.client.Logical().WriteWithContext(ctx, path, map[string]any{
		"custom_metadata": map[string]string{
			"owner":   "platform",
			"purpose": "observability-fixture",
		},
	})
	if err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("create namespace %s: %w", namespacePath, err)
	}
	r.addStep(stepName, path, "success", "")
	return nil
}

func namespaceStepSuffix(namespacePath string) string {
	return strings.ReplaceAll(strings.Trim(namespacePath, "/"), "/", "-")
}

func (r *scenarioRunner) exerciseNamespaceUserpass(ctx context.Context) error {
	config := baoapi.DefaultConfig()
	config.Address = r.client.Address()
	userClient, err := baoapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("create namespace userpass client: %w", err)
	}
	userClient.SetNamespace(fixtureNamespace)

	secret, err := userClient.Logical().WriteWithContext(ctx, "auth/userpass/login/namespace-user", map[string]any{
		"password": defaultScenarioPassword,
	})
	if err != nil {
		r.addStep("login-namespace-userpass", "auth/userpass/login/namespace-user", "error", err.Error())
		return fmt.Errorf("log in with namespace userpass user: %w", err)
	}
	token, err := secret.TokenID()
	if err != nil {
		r.addStep("login-namespace-userpass", "auth/userpass/login/namespace-user", "error", err.Error())
		return fmt.Errorf("read namespace userpass token: %w", err)
	}
	if token == "" {
		err := errors.New("namespace login response did not include a client token")
		r.addStep("login-namespace-userpass", "auth/userpass/login/namespace-user", "error", err.Error())
		return err
	}
	r.addStep("login-namespace-userpass", "auth/userpass/login/namespace-user", "success", "")

	userClient.SetToken(token)
	if _, err := userClient.Logical().ReadWithContext(ctx, "secret/data/apps/team-a/scenario"); err != nil {
		r.addStep("read-namespace-kv-as-user", "secret/data/apps/team-a/scenario", "error", err.Error())
		return fmt.Errorf("read namespace KV with namespace user token: %w", err)
	}
	r.addStep("read-namespace-kv-as-user", "secret/data/apps/team-a/scenario", "success", "")
	return nil
}

func (r *scenarioRunner) exerciseNamespaceToken(ctx context.Context) error {
	secret, err := r.client.Logical().WriteWithContext(ctx, "auth/token/create", map[string]any{
		"display_name": "namespace-scenario-token",
		"policies":     []string{"namespace-app"},
		"renewable":    true,
		"ttl":          "5m",
	})
	if err != nil {
		r.addStep("create-namespace-token", "auth/token/create", "error", err.Error())
		return fmt.Errorf("create namespace token: %w", err)
	}
	r.addStep("create-namespace-token", "auth/token/create", "success", "")

	token, err := secret.TokenID()
	if err != nil {
		return fmt.Errorf("read namespace token ID: %w", err)
	}
	if token == "" {
		return errors.New("namespace token create response did not include a token")
	}

	steps := []scenarioOperation{
		{
			name:      "lookup-namespace-token",
			path:      "auth/token/lookup",
			operation: "write",
			data:      map[string]any{"token": token},
		},
		{
			name:      "revoke-namespace-token",
			path:      "auth/token/revoke",
			operation: "write",
			data:      map[string]any{"token": token},
		},
	}
	for _, step := range steps {
		if err := r.runOperation(ctx, step); err != nil {
			return err
		}
	}
	return nil
}

func (r *scenarioRunner) exerciseNamespaceAppRole(ctx context.Context) error {
	if err := r.ensureAuth(
		ctx,
		"approle",
		"approle",
		nsAppRoleDesc,
		"ensure-namespace-approle-auth",
	); err != nil {
		return err
	}

	if err := r.runOperation(ctx, scenarioOperation{
		name:      "write-namespace-approle",
		path:      "auth/approle/role/namespace-app",
		operation: "write",
		data: map[string]any{
			"token_policies":     []string{"namespace-app"},
			"token_ttl":          "15m",
			"token_max_ttl":      "1h",
			"secret_id_ttl":      "30m",
			"secret_id_num_uses": 5,
		},
	}); err != nil {
		return err
	}

	roleIDSecret, err := r.client.Logical().ReadWithContext(ctx, "auth/approle/role/namespace-app/role-id")
	if err != nil {
		r.addStep("read-namespace-approle-role-id", "auth/approle/role/namespace-app/role-id", "error", err.Error())
		return fmt.Errorf("read namespace AppRole role ID: %w", err)
	}
	r.addStep("read-namespace-approle-role-id", "auth/approle/role/namespace-app/role-id", "success", "")

	secretIDSecret, err := r.client.Logical().WriteWithContext(ctx, "auth/approle/role/namespace-app/secret-id", nil)
	if err != nil {
		r.addStep(
			"write-namespace-approle-secret-id",
			"auth/approle/role/namespace-app/secret-id",
			"error",
			err.Error(),
		)
		return fmt.Errorf("create namespace AppRole secret ID: %w", err)
	}
	r.addStep("write-namespace-approle-secret-id", "auth/approle/role/namespace-app/secret-id", "success", "")

	roleID, ok := roleIDSecret.Data["role_id"].(string)
	if !ok || roleID == "" {
		return errors.New("namespace AppRole role-id response did not include data.role_id")
	}
	secretID, ok := secretIDSecret.Data["secret_id"].(string)
	if !ok || secretID == "" {
		return errors.New("namespace AppRole secret-id response did not include data.secret_id")
	}

	config := baoapi.DefaultConfig()
	config.Address = r.client.Address()
	appRoleClient, err := baoapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("create namespace AppRole client: %w", err)
	}
	appRoleClient.SetNamespace(fixtureNamespace)

	secret, err := appRoleClient.Logical().WriteWithContext(ctx, "auth/approle/login", map[string]any{
		"role_id":   roleID,
		"secret_id": secretID,
	})
	if err != nil {
		r.addStep("login-namespace-approle", "auth/approle/login", "error", err.Error())
		return fmt.Errorf("log in with namespace AppRole: %w", err)
	}
	token, err := secret.TokenID()
	if err != nil {
		r.addStep("login-namespace-approle", "auth/approle/login", "error", err.Error())
		return fmt.Errorf("read namespace AppRole token: %w", err)
	}
	if token == "" {
		err := errors.New("namespace AppRole login response did not include a client token")
		r.addStep("login-namespace-approle", "auth/approle/login", "error", err.Error())
		return err
	}
	r.addStep("login-namespace-approle", "auth/approle/login", "success", "")

	appRoleClient.SetToken(token)
	if _, err := appRoleClient.Logical().ReadWithContext(ctx, "secret/data/apps/team-a/scenario"); err != nil {
		r.addStep("read-namespace-kv-as-approle", "secret/data/apps/team-a/scenario", "error", err.Error())
		return fmt.Errorf("read namespace KV with namespace AppRole token: %w", err)
	}
	r.addStep("read-namespace-kv-as-approle", "secret/data/apps/team-a/scenario", "success", "")
	return nil
}

func (r *scenarioRunner) exerciseNamespaceDatabaseLease(ctx context.Context) error {
	if err := r.ensureMount(ctx, "database", "database", nsDBDesc, nil); err != nil {
		return err
	}
	r.renameLastStep("ensure-database-mount", "ensure-namespace-database-mount")
	if err := r.configurePostgresDatabase(ctx, "configure-namespace-postgres-database"); err != nil {
		return err
	}
	if err := r.writeDatabaseRole(
		ctx,
		"write-namespace-database-readonly-role",
		"readonly",
		postgresCreationStatements,
		postgresRevocationStatements,
		nil,
	); err != nil {
		return err
	}
	return r.exerciseDatabaseLeaseLifecycle(ctx, databaseLeaseLifecycle{
		credentialsPath: "database/creds/readonly",
		readStep:        "read-namespace-database-credentials",
		lookupStep:      "lookup-namespace-database-lease",
		renewStep:       "renew-namespace-database-lease",
		revokeStep:      "revoke-namespace-database-lease",
	})
}

func (r *scenarioRunner) exerciseNestedNamespace(ctx context.Context) error {
	if err := r.ensureNamespace(ctx, "payments"); err != nil {
		return err
	}
	r.renameLastStep("ensure-namespace-payments", "ensure-nested-namespace-payments")

	config := baoapi.DefaultConfig()
	config.Address = r.client.Address()
	nestedClient, err := baoapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("create nested namespace client: %w", err)
	}
	nestedClient.SetToken(r.client.Token())
	nestedClient.SetNamespace(fixtureNestedNamespace)

	return r.withClient(nestedClient, func() error {
		if err := r.ensureMount(
			ctx,
			"secret",
			"kv",
			nestedKVDesc,
			map[string]any{"version": "2"},
		); err != nil {
			return err
		}
		r.renameLastStep("ensure-secret-mount", "ensure-nested-namespace-secret-mount")

		steps := []scenarioOperation{
			{
				name:      "write-nested-namespace-kv",
				path:      "secret/data/apps/payments/scenario",
				operation: "write",
				data: map[string]any{
					"data": map[string]any{
						"owner":     "payments",
						"namespace": fixtureNestedNamespace,
						"scenario":  "nested-namespace-fixture",
					},
				},
			},
			{name: "read-nested-namespace-kv", path: "secret/data/apps/payments/scenario", operation: "read"},
			{name: "list-nested-namespace-kv-metadata", path: "secret/metadata/apps/payments", operation: "list"},
		}
		for _, step := range steps {
			if err := r.runOperation(ctx, step); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *scenarioRunner) exerciseNamespaceTransit(ctx context.Context) error {
	if err := r.ensureMount(ctx, "transit", "transit", nsTransitDesc, nil); err != nil {
		return err
	}
	r.renameLastStep("ensure-transit-mount", "ensure-namespace-transit-mount")
	if err := r.ensureTransitKey(ctx, "team-a", "ensure-namespace-transit-key"); err != nil {
		return err
	}

	key, err := r.client.Logical().ReadWithContext(ctx, "transit/keys/team-a")
	if err != nil {
		r.addStep("read-namespace-transit-key", "transit/keys/team-a", "error", err.Error())
		return fmt.Errorf("read namespace transit key: %w", err)
	}
	if key == nil {
		r.addStep("read-namespace-transit-key", "transit/keys/team-a", "error", "missing key")
		return errors.New("namespace transit key team-a does not exist")
	}
	r.addStep("read-namespace-transit-key", "transit/keys/team-a", "success", "")

	plaintext := "openbao-observability-namespace-transit-scenario"
	encrypted, err := r.client.Logical().WriteWithContext(ctx, "transit/encrypt/team-a", map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString([]byte(plaintext)),
	})
	if err != nil {
		r.addStep("encrypt-namespace-transit", "transit/encrypt/team-a", "error", err.Error())
		return fmt.Errorf("encrypt namespace transit payload: %w", err)
	}
	if encrypted == nil || encrypted.Data == nil {
		err := errors.New("namespace transit encrypt response did not include data")
		r.addStep("encrypt-namespace-transit", "transit/encrypt/team-a", "error", err.Error())
		return err
	}
	ciphertext, ok := encrypted.Data["ciphertext"].(string)
	if !ok || ciphertext == "" {
		err := errors.New("namespace transit encrypt response did not include data.ciphertext")
		r.addStep("encrypt-namespace-transit", "transit/encrypt/team-a", "error", err.Error())
		return err
	}
	r.addStep("encrypt-namespace-transit", "transit/encrypt/team-a", "success", "")

	decrypted, err := r.client.Logical().WriteWithContext(ctx, "transit/decrypt/team-a", map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		r.addStep("decrypt-namespace-transit", "transit/decrypt/team-a", "error", err.Error())
		return fmt.Errorf("decrypt namespace transit payload: %w", err)
	}
	if decrypted == nil || decrypted.Data == nil {
		err := errors.New("namespace transit decrypt response did not include data")
		r.addStep("decrypt-namespace-transit", "transit/decrypt/team-a", "error", err.Error())
		return err
	}
	encodedPlaintext, ok := decrypted.Data["plaintext"].(string)
	if !ok || encodedPlaintext == "" {
		err := errors.New("namespace transit decrypt response did not include data.plaintext")
		r.addStep("decrypt-namespace-transit", "transit/decrypt/team-a", "error", err.Error())
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(encodedPlaintext)
	if err != nil {
		r.addStep("decrypt-namespace-transit", "transit/decrypt/team-a", "error", err.Error())
		return fmt.Errorf("decode namespace transit plaintext: %w", err)
	}
	if string(decoded) != plaintext {
		err := errors.New("namespace transit decrypt plaintext did not match original payload")
		r.addStep("decrypt-namespace-transit", "transit/decrypt/team-a", "error", err.Error())
		return err
	}
	r.addStep("decrypt-namespace-transit", "transit/decrypt/team-a", "success", "")
	return nil
}

func (r *scenarioRunner) exerciseNamespacePKI(ctx context.Context) error {
	if err := r.ensureMount(ctx, "pki", "pki", nsPKIDesc, nil); err != nil {
		return err
	}
	r.renameLastStep("ensure-pki-mount", "ensure-namespace-pki-mount")
	if err := r.ensurePKIRootNamed(ctx, "ensure-namespace-pki-root"); err != nil {
		return err
	}

	roleName := "team-a-dot-observability-dot-local"
	rolePath := "pki/roles/" + roleName
	if err := r.runOperation(ctx, scenarioOperation{
		name:      "write-namespace-pki-role",
		path:      rolePath,
		operation: "write",
		data: map[string]any{
			"allowed_domains":    []string{"team-a.observability.local"},
			"allow_bare_domains": true,
			"allow_subdomains":   true,
			"max_ttl":            "1h",
		},
	}); err != nil {
		return err
	}

	issuePath := "pki/issue/" + roleName
	issued, err := r.client.Logical().WriteWithContext(ctx, issuePath, map[string]any{
		"common_name": "app.team-a.observability.local",
		"ttl":         "30m",
	})
	if err != nil {
		r.addStep("issue-namespace-pki-certificate", issuePath, "error", err.Error())
		return fmt.Errorf("issue namespace PKI certificate: %w", err)
	}
	if issued == nil || issued.Data == nil {
		err := errors.New("namespace PKI issue response did not include data")
		r.addStep("issue-namespace-pki-certificate", issuePath, "error", err.Error())
		return err
	}
	serial, ok := issued.Data["serial_number"].(string)
	if !ok || serial == "" {
		err := errors.New("namespace PKI issue response did not include data.serial_number")
		r.addStep("issue-namespace-pki-certificate", issuePath, "error", err.Error())
		return err
	}
	r.addStep("issue-namespace-pki-certificate", issuePath, "success", "serial_number_present")

	if _, err := r.client.Logical().WriteWithContext(ctx, "pki/revoke", map[string]any{
		"serial_number": serial,
	}); err != nil {
		r.addStep("revoke-namespace-pki-certificate", "pki/revoke", "error", err.Error())
		return fmt.Errorf("revoke namespace PKI certificate: %w", err)
	}
	r.addStep("revoke-namespace-pki-certificate", "pki/revoke", "success", "")

	if err := r.expectWriteError(ctx, "failed-namespace-pki-issue-invalid-domain", issuePath, map[string]any{
		"common_name": "team-a.unapproved.example",
		"ttl":         "30m",
	}); err != nil {
		return err
	}
	if err := r.expectWriteError(ctx, "failed-namespace-pki-revoke-invalid-serial", "pki/revoke", map[string]any{
		"serial_number": "not-a-valid-serial",
	}); err != nil {
		return err
	}

	return nil
}

func (r *scenarioRunner) ensureAuditCanary(ctx context.Context) error {
	canarySteps := []scenarioOperation{
		{
			name:      "write-audit-canary-policy",
			path:      "sys/policies/acl/audit-canary",
			operation: "write",
			data: map[string]any{
				"policy": `path "secret/data/observability/audit-canary" {
  capabilities = ["read"]
}
`,
			},
		},
		{name: "write-audit-canary-secret", path: auditCanaryPath, operation: "write", data: map[string]any{
			"data": map[string]any{
				"owner":   "observability",
				"purpose": "audit-canary",
				"status":  "ok",
			},
		}},
	}
	for _, step := range canarySteps {
		if err := r.runOperation(ctx, step); err != nil {
			return err
		}
	}

	if _, err := r.client.Logical().WriteWithContext(ctx, "auth/token/lookup", map[string]any{
		"token": auditCanaryToken,
	}); err != nil {
		if _, createErr := r.client.Logical().WriteWithContext(ctx, "auth/token/create-orphan", map[string]any{
			"id":                auditCanaryToken,
			"display_name":      "audit-canary",
			"policies":          []string{"audit-canary"},
			"no_default_policy": true,
		}); createErr != nil {
			r.addStep("create-audit-canary-token", "auth/token/create-orphan", "error", createErr.Error())
			return fmt.Errorf("create audit canary token: %w", createErr)
		}
		r.addStep("create-audit-canary-token", "auth/token/create-orphan", "success", "")
	} else {
		r.addStep("lookup-audit-canary-token", "auth/token/lookup", "success", "")
	}

	config := baoapi.DefaultConfig()
	config.Address = r.client.Address()
	canaryClient, err := baoapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("create audit canary client: %w", err)
	}
	canaryClient.SetToken(auditCanaryToken)
	if _, err := canaryClient.Logical().ReadWithContext(ctx, auditCanaryPath); err != nil {
		r.addStep("read-audit-canary", auditCanaryPath, "error", err.Error())
		return fmt.Errorf("read audit canary with canary token: %w", err)
	}
	r.addStep("read-audit-canary", auditCanaryPath, "success", "")
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
	case "delete":
		_, err = r.client.Logical().DeleteWithContext(ctx, step.path)
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

	secretIDSecret, err := r.client.Logical().
		WriteWithContext(ctx, "auth/approle/role/observability-app/secret-id", nil)
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
		{
			name:      "renew-token",
			path:      "auth/token/renew",
			operation: "write",
			data:      map[string]any{"token": token, "increment": "60s"},
		},
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
	return r.exerciseDatabaseLeaseLifecycle(ctx, databaseLeaseLifecycle{
		credentialsPath: "database/creds/readonly",
		readStep:        "read-database-credentials",
		lookupStep:      "lookup-database-lease",
		renewStep:       "renew-database-lease",
		revokeStep:      "revoke-database-lease",
	})
}

type databaseLeaseLifecycle struct {
	credentialsPath string
	readStep        string
	lookupStep      string
	renewStep       string
	revokeStep      string
}

func (r *scenarioRunner) exerciseDatabaseLeaseLifecycle(ctx context.Context, lifecycle databaseLeaseLifecycle) error {
	secret, err := r.client.Logical().ReadWithContext(ctx, lifecycle.credentialsPath)
	if err != nil {
		r.addStep(lifecycle.readStep, lifecycle.credentialsPath, "error", err.Error())
		return fmt.Errorf("read dynamic database credentials: %w", err)
	}
	if secret == nil || secret.LeaseID == "" {
		r.addStep(lifecycle.readStep, lifecycle.credentialsPath, "error", "missing lease_id")
		return errors.New("database credentials response did not include a lease_id")
	}
	r.addStep(lifecycle.readStep, lifecycle.credentialsPath, "success", "lease_id_present")

	leaseSteps := []struct {
		name string
		run  func(context.Context, string) error
	}{
		{name: lifecycle.lookupStep, run: func(ctx context.Context, leaseID string) error {
			_, err := r.client.Sys().LookupWithContext(ctx, leaseID)
			return err
		}},
		{name: lifecycle.renewStep, run: func(ctx context.Context, leaseID string) error {
			_, err := r.client.Sys().RenewWithContext(ctx, leaseID, int((2 * time.Minute).Seconds()))
			return err
		}},
		{name: lifecycle.revokeStep, run: func(ctx context.Context, leaseID string) error {
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

func (r *scenarioRunner) exerciseKVv1(ctx context.Context) error {
	if err := r.ensureMount(ctx, "kv-v1", "kv", scenarioKVDesc, map[string]any{
		"version": "1",
	}); err != nil {
		return err
	}

	steps := []scenarioOperation{
		{name: "write-kv-v1", path: "kv-v1/apps/payments/config", operation: "write", data: map[string]any{
			"owner":    "payments",
			"scenario": "kv-v1-fixture",
			"tier":     "demo",
		}},
		{name: "read-kv-v1", path: "kv-v1/apps/payments/config", operation: "read"},
		{name: "list-kv-v1", path: "kv-v1/apps/payments", operation: "list"},
		{name: "delete-kv-v1", path: "kv-v1/apps/payments/config", operation: "delete"},
	}
	for _, step := range steps {
		if err := r.runOperation(ctx, step); err != nil {
			return err
		}
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

func (r *scenarioRunner) exercisePKI(ctx context.Context) error {
	if err := r.ensureMount(ctx, "pki", "pki", scenarioPKIDesc, nil); err != nil {
		return err
	}
	if err := r.ensurePKIRoot(ctx); err != nil {
		return err
	}

	if _, err := r.client.Logical().WriteWithContext(ctx, "pki/roles/observability-dot-local", map[string]any{
		"allowed_domains":    []string{"observability.local"},
		"allow_bare_domains": true,
		"allow_subdomains":   true,
		"max_ttl":            "1h",
	}); err != nil {
		r.addStep("write-pki-role", "pki/roles/observability-dot-local", "error", err.Error())
		return fmt.Errorf("write PKI role: %w", err)
	}
	r.addStep("write-pki-role", "pki/roles/observability-dot-local", "success", "")

	issued, err := r.client.Logical().WriteWithContext(ctx, "pki/issue/observability-dot-local", map[string]any{
		"common_name": "payments.observability.local",
		"ttl":         "30m",
	})
	if err != nil {
		r.addStep("issue-pki-certificate", "pki/issue/observability-dot-local", "error", err.Error())
		return fmt.Errorf("issue PKI certificate: %w", err)
	}
	if issued == nil || issued.Data == nil {
		err := errors.New("PKI issue response did not include data")
		r.addStep("issue-pki-certificate", "pki/issue/observability-dot-local", "error", err.Error())
		return err
	}
	serial, ok := issued.Data["serial_number"].(string)
	if !ok || serial == "" {
		err := errors.New("PKI issue response did not include data.serial_number")
		r.addStep("issue-pki-certificate", "pki/issue/observability-dot-local", "error", err.Error())
		return err
	}
	r.addStep("issue-pki-certificate", "pki/issue/observability-dot-local", "success", "serial_number_present")

	if _, err := r.client.Logical().WriteWithContext(ctx, "pki/revoke", map[string]any{
		"serial_number": serial,
	}); err != nil {
		r.addStep("revoke-pki-certificate", "pki/revoke", "error", err.Error())
		return fmt.Errorf("revoke PKI certificate: %w", err)
	}
	r.addStep("revoke-pki-certificate", "pki/revoke", "success", "")
	return nil
}

func (r *scenarioRunner) exerciseFeatureExpectedFailures(ctx context.Context) error {
	if err := r.expectWriteError(ctx, "failed-pki-issue-invalid-domain",
		"pki/issue/observability-dot-local", map[string]any{
			"common_name": "payments.unapproved.example",
			"ttl":         "30m",
		}); err != nil {
		return err
	}
	if err := r.expectWriteError(ctx, "failed-pki-revoke-invalid-serial", "pki/revoke", map[string]any{
		"serial_number": "not-a-valid-serial",
	}); err != nil {
		return err
	}
	if err := r.exerciseDatabaseExpectedFailures(ctx); err != nil {
		return err
	}
	return nil
}

func (r *scenarioRunner) exerciseDatabaseExpectedFailures(ctx context.Context) error {
	if err := r.expectWriteError(ctx, "failed-database-initialize", "database/config/postgres-bad", map[string]any{
		"plugin_name":    "postgresql-database-plugin",
		"allowed_roles":  []string{"readonly"},
		"connection_url": "postgresql://{{username}}:{{password}}@127.0.0.1:1/openbao_app?sslmode=disable",
		"username":       fixturePostgresUser,
		"password":       fixturePostgresPassword,
	}); err != nil {
		return err
	}

	if err := r.runOperation(ctx, scenarioOperation{
		name:      "write-database-failure-create-role",
		path:      "database/roles/failure-create",
		operation: "write",
		data: map[string]any{
			"db_name":             "postgres",
			"default_ttl":         "5m",
			"max_ttl":             "30m",
			"creation_statements": postgresInvalidStatement,
		},
	}); err != nil {
		return err
	}
	if err := r.expectReadError(ctx, "failed-database-new-user", "database/creds/failure-create"); err != nil {
		return err
	}

	renewLeaseID, err := r.issueDatabaseLease(ctx, "failure-renew", "read-database-failure-renew-credentials")
	if err != nil {
		return err
	}
	if err := r.writeDatabaseRole(
		ctx,
		"write-database-failure-renew-invalid-role",
		"failure-renew",
		postgresCreationStatements,
		postgresRevocationStatements,
		postgresInvalidStatement,
	); err != nil {
		return err
	}
	if err := r.expectLeaseRenewError(ctx, "failed-database-update-user", renewLeaseID); err != nil {
		return err
	}
	if err := r.writeDatabaseRole(
		ctx,
		"write-database-failure-renew-recovery-role",
		"failure-renew",
		postgresCreationStatements,
		postgresRevocationStatements,
		nil,
	); err != nil {
		return err
	}
	if err := r.client.Sys().RevokeWithContext(ctx, renewLeaseID); err != nil {
		r.addStep("revoke-database-failure-renew-lease", "sys/leases/revoke", "error", err.Error())
		return fmt.Errorf("revoke database renew failure lease: %w", err)
	}
	r.addStep("revoke-database-failure-renew-lease", "sys/leases/revoke", "success", "")

	revokeLeaseID, err := r.issueDatabaseLease(ctx, "failure-revoke", "read-database-failure-revoke-credentials")
	if err != nil {
		return err
	}
	if err := r.writeDatabaseRole(
		ctx,
		"write-database-failure-revoke-invalid-role",
		"failure-revoke",
		postgresCreationStatements,
		postgresInvalidStatement,
		nil,
	); err != nil {
		return err
	}
	if err := r.expectLeaseRevokeError(ctx, "failed-database-delete-user", revokeLeaseID); err != nil {
		return err
	}
	if err := r.writeDatabaseRole(
		ctx,
		"write-database-failure-revoke-recovery-role",
		"failure-revoke",
		postgresCreationStatements,
		postgresRevocationStatements,
		nil,
	); err != nil {
		return err
	}
	if err := r.client.Sys().RevokeWithContext(ctx, revokeLeaseID); err != nil {
		r.addStep("revoke-database-failure-revoke-lease", "sys/leases/revoke", "error", err.Error())
		return fmt.Errorf("revoke database delete failure lease after recovery: %w", err)
	}
	r.addStep("revoke-database-failure-revoke-lease", "sys/leases/revoke", "success", "")

	return nil
}

func (r *scenarioRunner) configurePostgresDatabase(ctx context.Context, stepName string) error {
	return r.runOperation(ctx, scenarioOperation{
		name:      stepName,
		path:      "database/config/postgres",
		operation: "write",
		data: map[string]any{
			"plugin_name":    "postgresql-database-plugin",
			"allowed_roles":  []string{"readonly", "failure-*"},
			"connection_url": postgresConnectionURL(r.postgresHost),
			"username":       fixturePostgresUser,
			"password":       fixturePostgresPassword,
		},
	})
}

func postgresConnectionURL(host string) string {
	return fmt.Sprintf("postgresql://{{username}}:{{password}}@%s:5432/%s?sslmode=disable", host, fixturePostgresDB)
}

func (r *scenarioRunner) issueDatabaseLease(ctx context.Context, roleName, stepName string) (string, error) {
	if err := r.writeDatabaseRole(
		ctx,
		"write-database-"+roleName+"-role",
		roleName,
		postgresCreationStatements,
		postgresRevocationStatements,
		nil,
	); err != nil {
		return "", err
	}
	secret, err := r.client.Logical().ReadWithContext(ctx, "database/creds/"+roleName)
	if err != nil {
		r.addStep(stepName, "database/creds/"+roleName, "error", err.Error())
		return "", fmt.Errorf("read dynamic database credentials for %s: %w", roleName, err)
	}
	if secret == nil || secret.LeaseID == "" {
		r.addStep(stepName, "database/creds/"+roleName, "error", "missing lease_id")
		return "", fmt.Errorf("database credentials response for %s did not include a lease_id", roleName)
	}
	r.addStep(stepName, "database/creds/"+roleName, "success", "lease_id_present")
	return secret.LeaseID, nil
}

func (r *scenarioRunner) writeDatabaseRole(
	ctx context.Context,
	stepName, roleName string,
	creationStatements, revocationStatements, renewStatements []string,
) error {
	data := map[string]any{
		"db_name":             "postgres",
		"default_ttl":         "5m",
		"max_ttl":             "30m",
		"creation_statements": creationStatements,
	}
	if revocationStatements != nil {
		data["revocation_statements"] = revocationStatements
	}
	if renewStatements != nil {
		data["renew_statements"] = renewStatements
	}
	return r.runOperation(ctx, scenarioOperation{
		name:      stepName,
		path:      "database/roles/" + roleName,
		operation: "write",
		data:      data,
	})
}

func (r *scenarioRunner) expectReadError(ctx context.Context, name, path string) error {
	if _, err := r.client.Logical().ReadWithContext(ctx, path); err != nil {
		r.addStep(name, path, "expected_error", "")
		return nil
	}
	return fmt.Errorf("%s unexpectedly succeeded", name)
}

func (r *scenarioRunner) expectWriteError(ctx context.Context, name, path string, data map[string]any) error {
	if _, err := r.client.Logical().WriteWithContext(ctx, path, data); err != nil {
		r.addStep(name, path, "expected_error", "")
		return nil
	}
	return fmt.Errorf("%s unexpectedly succeeded", name)
}

func (r *scenarioRunner) expectLeaseRenewError(ctx context.Context, name, leaseID string) error {
	if _, err := r.client.Sys().RenewWithContext(ctx, leaseID, int((2 * time.Minute).Seconds())); err != nil {
		r.addStep(name, "sys/leases/renew", "expected_error", "")
		return nil
	}
	return fmt.Errorf("%s unexpectedly succeeded", name)
}

func (r *scenarioRunner) expectLeaseRevokeError(ctx context.Context, name, leaseID string) error {
	if err := r.client.Sys().RevokeWithContext(ctx, leaseID); err != nil {
		r.addStep(name, "sys/leases/revoke", "expected_error", "")
		return nil
	}
	return fmt.Errorf("%s unexpectedly succeeded", name)
}

func (r *scenarioRunner) ensurePKIRoot(ctx context.Context) error {
	return r.ensurePKIRootNamed(ctx, "ensure-pki-root")
}

func (r *scenarioRunner) ensurePKIRootNamed(ctx context.Context, stepName string) error {
	cert, err := r.client.Logical().ReadWithContext(ctx, "pki/cert/ca")
	if err == nil && cert != nil {
		r.addStep(stepName, "pki/cert/ca", "success", "")
		return nil
	}

	generated, err := r.client.Logical().WriteWithContext(ctx, "pki/root/generate/internal", map[string]any{
		"common_name": "OpenBao Observability Fixture Root CA",
		"ttl":         "24h",
	})
	if err != nil {
		r.addStep(stepName, "pki/root/generate/internal", "error", err.Error())
		return fmt.Errorf("generate PKI root: %w", err)
	}
	if generated == nil || generated.Data == nil {
		err := errors.New("PKI root generation response did not include data")
		r.addStep(stepName, "pki/root/generate/internal", "error", err.Error())
		return err
	}
	r.addStep(stepName, "pki/root/generate/internal", "success", "")
	return nil
}

func (r *scenarioRunner) ensureTransitMount(ctx context.Context) error {
	if err := r.ensureMount(ctx, "transit", "transit", scenarioTransitDesc, nil); err != nil {
		return err
	}
	return r.ensureTransitKey(ctx, "payments", "ensure-transit-key")
}

func (r *scenarioRunner) ensureTransitKey(ctx context.Context, keyName, stepName string) error {
	path := "transit/keys/" + keyName
	key, err := r.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("read transit key before scenario: %w", err)
	}
	if key != nil {
		r.addStep(stepName, path, "success", "")
		return nil
	}

	if _, err := r.client.Logical().WriteWithContext(ctx, path, map[string]any{
		"type":                   "aes256-gcm96",
		"derived":                false,
		"exportable":             false,
		"allow_plaintext_backup": false,
	}); err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("create transit key: %w", err)
	}
	r.addStep(stepName, path, "success", "")
	return nil
}

func (r *scenarioRunner) ensureMount(
	ctx context.Context,
	mountPath, engineType, description string,
	options map[string]any,
) error {
	mounts, err := r.client.Logical().ReadWithContext(ctx, "sys/mounts")
	if err != nil {
		r.addStep("ensure-"+mountPath+"-mount", "sys/mounts", "error", err.Error())
		return fmt.Errorf("read mounts before %s scenario: %w", mountPath, err)
	}
	if mounts == nil || mounts.Data == nil {
		err := errors.New("sys/mounts response did not include data")
		r.addStep("ensure-"+mountPath+"-mount", "sys/mounts", "error", err.Error())
		return err
	}

	stepName := "ensure-" + mountPath + "-mount"
	if _, ok := mounts.Data[mountPath+"/"]; ok {
		r.addStep(stepName, "sys/mounts", "success", "")
		return nil
	}

	data := map[string]any{
		"type":        engineType,
		"description": description,
	}
	if len(options) > 0 {
		data["options"] = options
	}
	path := "sys/mounts/" + mountPath
	if _, err := r.client.Logical().WriteWithContext(ctx, path, data); err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("enable %s mount: %w", mountPath, err)
	}
	r.addStep(stepName, path, "success", "")
	return nil
}

func (r *scenarioRunner) ensureAuth(ctx context.Context, mountPath, authType, description, stepName string) error {
	auths, err := r.client.Logical().ReadWithContext(ctx, "sys/auth")
	if err != nil {
		r.addStep(stepName, "sys/auth", "error", err.Error())
		return fmt.Errorf("read auth mounts before %s scenario: %w", mountPath, err)
	}
	if auths == nil || auths.Data == nil {
		err := errors.New("sys/auth response did not include data")
		r.addStep(stepName, "sys/auth", "error", err.Error())
		return err
	}

	if _, ok := auths.Data[mountPath+"/"]; ok {
		r.addStep(stepName, "sys/auth", "success", "")
		return nil
	}

	path := "sys/auth/" + mountPath
	if _, err := r.client.Logical().WriteWithContext(ctx, path, map[string]any{
		"type":        authType,
		"description": description,
	}); err != nil {
		r.addStep(stepName, path, "error", err.Error())
		return fmt.Errorf("enable auth mount %s: %w", mountPath, err)
	}
	r.addStep(stepName, path, "success", "")
	return nil
}

func (r *scenarioRunner) withClient(client *baoapi.Client, run func() error) error {
	previous := r.client
	r.client = client
	defer func() {
		r.client = previous
	}()
	return run()
}

func (r *scenarioRunner) renameLastStep(from, to string) {
	if len(r.report.Steps) == 0 {
		return
	}
	last := &r.report.Steps[len(r.report.Steps)-1]
	if last.Name == from {
		last.Name = to
	}
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
	if err := readerRunner.loginUserpass(
		ctx,
		"demo-reader",
		defaultScenarioPassword,
		"login-userpass-reader",
	); err != nil {
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
