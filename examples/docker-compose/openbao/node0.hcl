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

  request "create-demo-admin-user" {
    operation = "update"
    path      = "auth/userpass/users/demo-admin"

    data = {
      password       = "openbao-observability"
      token_policies = ["compose-admin"]
    }
  }

  request "create-local-metrics-token" {
    operation = "update"
    path      = "auth/token/create-orphan"

    data = {
      id       = "openbao-observability-metrics-token"
      policies = ["openbao-metrics"]
    }
  }
}
