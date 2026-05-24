# Fixtures

This directory contains captured OpenBao metrics and log fixtures.

Generated captures live under `fixtures/captured/` and are ignored by Git by
default because they contain timestamps, generated cluster IDs, and local
command output. Run `make fixtures-openbao` to regenerate them.

The HA/Raft capture includes production-like activity for root namespace
operations, one child namespace, one minimal nested namespace, database lease
lookup/renew/revoke behavior, audit logs, and all-node metrics.
