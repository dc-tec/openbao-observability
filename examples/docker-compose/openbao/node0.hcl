ui            = true
disable_mlock = true

cluster_name = "openbao-observability-compose"
api_addr     = "http://openbao-node0:8200"
cluster_addr = "http://openbao-node0:8201"

log_level  = "info"
log_format = "json"
log_file   = "/openbao/logs/openbao.log"

storage "raft" {
  path                   = "/openbao/data"
  node_id                = "node0"
  performance_multiplier = 1
}

listener "tcp" {
  address         = "0.0.0.0:8200"
  cluster_address = "0.0.0.0:8201"
  tls_disable     = true

  telemetry {
    unauthenticated_metrics_access = true
  }
}

seal "static" {
  current_key_id = "local-1"
  current_key    = "file:///openbao/seal/static-unseal.key"
}

telemetry {
  prometheus_retention_time = "30s"
  disable_hostname          = true
  metrics_prefix            = "vault"
}

audit "file" "local-file" {
  description = "Docker Compose audit stream."

  options {
    file_path     = "/openbao/audit/audit.jsonl"
    mode          = "0600"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}

initialize "compose-foundation" {
  request "enable-userpass-auth" {
    operation = "update"
    path      = "sys/auth/userpass"

    data = {
      type = "userpass"
    }
  }

  request "enable-approle-auth" {
    operation = "update"
    path      = "sys/auth/approle"

    data = {
      type        = "approle"
      description = "Local Compose AppRole auth method for observability testing."
    }
  }

  request "enable-secret-kv-v2" {
    operation = "update"
    path      = "sys/mounts/secret"

    data = {
      type        = "kv"
      description = "Local Compose KV engine for observability testing."

      options = {
        version = "2"
      }
    }
  }

  request "enable-database-secrets" {
    operation = "update"
    path      = "sys/mounts/database"

    data = {
      type        = "database"
      description = "Local Compose PostgreSQL dynamic secrets engine."
    }
  }

  request "configure-postgres-database" {
    operation = "update"
    path      = "database/config/postgres"

    data = {
      plugin_name    = "postgresql-database-plugin"
      allowed_roles  = ["readonly"]
      connection_url = "postgresql://{{username}}:{{password}}@postgres:5432/openbao_app?sslmode=disable"
      username       = "openbao_admin"
      password       = "openbao_admin_password"
    }
  }

  request "create-postgres-readonly-role" {
    operation = "update"
    path      = "database/roles/readonly"

    data = {
      db_name     = "postgres"
      default_ttl = "5m"
      max_ttl     = "30m"

      creation_statements = [
        "CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';",
        "GRANT CONNECT ON DATABASE openbao_app TO \"{{name}}\";"
      ]
    }
  }

  request "create-compose-admin-policy" {
    operation = "update"
    path      = "sys/policies/acl/compose-admin"

    data = {
      policy = <<EOT
path "*" {
  capabilities = ["create", "read", "update", "delete", "list", "sudo"]
}
EOT
    }
  }

  request "create-app-reader-policy" {
    operation = "update"
    path      = "sys/policies/acl/app-reader"

    data = {
      policy = <<EOT
path "secret/data/apps/*" {
  capabilities = ["read"]
}

path "secret/metadata/apps/*" {
  capabilities = ["list", "read"]
}
EOT
    }
  }

  request "create-app-writer-policy" {
    operation = "update"
    path      = "sys/policies/acl/app-writer"

    data = {
      policy = <<EOT
path "secret/data/apps/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "secret/metadata/apps/*" {
  capabilities = ["list", "read", "delete"]
}
EOT
    }
  }

  request "create-identity-auditor-policy" {
    operation = "update"
    path      = "sys/policies/acl/identity-auditor"

    data = {
      policy = <<EOT
path "identity/*" {
  capabilities = ["read", "list"]
}

path "sys/auth" {
  capabilities = ["read", "list"]
}
EOT
    }
  }

  request "create-metrics-policy" {
    operation = "update"
    path      = "sys/policies/acl/openbao-metrics"

    data = {
      policy = <<EOT
path "sys/metrics" {
  capabilities = ["read", "list"]
}
EOT
    }
  }

  request "read-auth-methods" {
    operation = "read"
    path      = "sys/auth"
  }

  request "create-demo-admin-user" {
    operation = "update"
    path      = "auth/userpass/users/demo-admin"

    data = {
      password       = "openbao-observability"
      token_policies = ["compose-admin"]
    }
  }

  request "create-demo-reader-user" {
    operation = "update"
    path      = "auth/userpass/users/demo-reader"

    data = {
      password       = "openbao-observability"
      token_policies = ["app-reader"]
    }
  }

  request "create-demo-writer-user" {
    operation = "update"
    path      = "auth/userpass/users/demo-writer"

    data = {
      password       = "openbao-observability"
      token_policies = ["app-writer"]
    }
  }

  request "create-observability-approle" {
    operation = "update"
    path      = "auth/approle/role/observability-app"

    data = {
      token_policies    = ["app-reader"]
      token_ttl         = "15m"
      token_max_ttl     = "1h"
      secret_id_ttl     = "30m"
      secret_id_num_uses = 5
    }
  }

  request "create-observability-approle-secret-id" {
    operation = "update"
    path      = "auth/approle/role/observability-app/secret-id"
  }

  request "create-local-metrics-token" {
    operation = "update"
    path      = "auth/token/create-orphan"

    data = {
      id       = "openbao-observability-metrics-token"
      policies = ["openbao-metrics"]
    }
  }

  request "create-local-reader-token" {
    operation = "update"
    path      = "auth/token/create"

    data = {
      display_name = "compose-reader"
      policies     = ["app-reader"]
      ttl          = "5m"
    }
  }

  request "create-demo-admin-entity" {
    operation = "update"
    path      = "identity/entity"

    data = {
      name     = "demo-admin"
      policies = ["compose-admin"]

      metadata = {
        team        = "platform"
        environment = "local"
      }
    }
  }

  request "create-demo-reader-entity" {
    operation = "update"
    path      = "identity/entity"

    data = {
      name     = "demo-reader"
      policies = ["app-reader"]

      metadata = {
        team        = "payments"
        environment = "local"
      }
    }
  }

  request "create-demo-writer-entity" {
    operation = "update"
    path      = "identity/entity"

    data = {
      name     = "demo-writer"
      policies = ["app-writer"]

      metadata = {
        team        = "payments"
        environment = "local"
      }
    }
  }

  request "create-demo-admin-alias" {
    operation = "update"
    path      = "identity/entity-alias"

    data = {
      name = "demo-admin"

      canonical_id = {
        eval_source     = "response"
        eval_type       = "string"
        initialize_name = "compose-foundation"
        response_name   = "create-demo-admin-entity"
        field_selector  = ["data", "id"]
      }

      mount_accessor = {
        eval_source     = "response"
        eval_type       = "string"
        initialize_name = "compose-foundation"
        response_name   = "read-auth-methods"
        field_selector  = ["data", "userpass/", "accessor"]
      }
    }
  }

  request "create-demo-reader-alias" {
    operation = "update"
    path      = "identity/entity-alias"

    data = {
      name = "demo-reader"

      canonical_id = {
        eval_source     = "response"
        eval_type       = "string"
        initialize_name = "compose-foundation"
        response_name   = "create-demo-reader-entity"
        field_selector  = ["data", "id"]
      }

      mount_accessor = {
        eval_source     = "response"
        eval_type       = "string"
        initialize_name = "compose-foundation"
        response_name   = "read-auth-methods"
        field_selector  = ["data", "userpass/", "accessor"]
      }
    }
  }

  request "create-demo-writer-alias" {
    operation = "update"
    path      = "identity/entity-alias"

    data = {
      name = "demo-writer"

      canonical_id = {
        eval_source     = "response"
        eval_type       = "string"
        initialize_name = "compose-foundation"
        response_name   = "create-demo-writer-entity"
        field_selector  = ["data", "id"]
      }

      mount_accessor = {
        eval_source     = "response"
        eval_type       = "string"
        initialize_name = "compose-foundation"
        response_name   = "read-auth-methods"
        field_selector  = ["data", "userpass/", "accessor"]
      }
    }
  }

  request "create-platform-identity-group" {
    operation = "update"
    path      = "identity/group"

    data = {
      name     = "platform-team"
      type     = "internal"
      policies = ["identity-auditor"]

      metadata = {
        owner = "platform"
      }
    }
  }

  request "create-payments-identity-group" {
    operation = "update"
    path      = "identity/group"

    data = {
      name     = "payments-team"
      type     = "internal"
      policies = ["app-reader"]

      metadata = {
        owner = "payments"
      }
    }
  }

  request "write-payments-api-secret" {
    operation = "update"
    path      = "secret/data/apps/payments/api"

    data = {
      data = {
        owner    = "payments"
        rotation = "30d"
        tier     = "demo"
      }
    }
  }

  request "read-payments-api-secret" {
    operation = "read"
    path      = "secret/data/apps/payments/api"
  }
}
