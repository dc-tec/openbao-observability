# Dashboards

This directory contains dashboard documentation and dashboard-specific source
material that is not part of the contract system.

Dashboard contracts live in `contracts/dashboards/`. Generated Grafana JSON
artifacts live in `generated/grafana/`.

Dashboard contracts carry top-level maturity labels. Current dashboard
contracts use `maturity.lifecycle: draft` and `generated-validated` evidence;
live query results, data-source permissions, and production label shape remain
profile or environment checks.

Current generated dashboards:

- `OpenBao overview`
- `OpenBao HA/Raft`
- `OpenBao audit overview`
- `OpenBao operational logs`
- `OpenBao audit investigation`
- `OpenBao auth and identity`
- `OpenBao token and lease lifecycle`
- `OpenBao secret engines and mounts`
- `OpenBao runtime and storage`
