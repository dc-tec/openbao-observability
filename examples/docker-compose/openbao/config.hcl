ui            = true
disable_mlock = true

log_level  = "info"
log_format = "json"
log_file   = "/openbao/logs/openbao.log"

telemetry {
  prometheus_retention_time = "30s"
  disable_hostname          = true
  metrics_prefix            = "vault"
}

audit "file" "local-file" {
  description = "Docker Compose audit stream."

  options {
    file_path     = "/openbao/file/audit.jsonl"
    mode          = "0600"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
