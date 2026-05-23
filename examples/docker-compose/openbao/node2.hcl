ui            = true
disable_mlock = true

cluster_name = "openbao-observability-compose"
api_addr     = "http://openbao-node2:8200"
cluster_addr = "http://openbao-node2:8201"

log_level  = "info"
log_format = "json"
log_file   = "/openbao/logs/openbao.log"

storage "raft" {
  path                   = "/openbao/data"
  node_id                = "node2"
  performance_multiplier = 1

  retry_join {
    leader_api_addr = "http://openbao-node0:8200"
  }
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
