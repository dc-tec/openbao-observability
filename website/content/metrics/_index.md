---
title: "Metrics"
description: "Metric behavior, scrape profiles, compatibility, recording rules, HA/Raft metrics, and token and lease metrics."
weight: 40
browse:
  - "/metrics/understanding-openbao-metrics"
  - "/metrics/secure-metrics-scrape"
  - "/metrics/all-node-metrics-scrape"
  - "/metrics/compatibility-matrix"
  - "/metrics/ha-raft-metrics"
  - "/metrics/token-and-lease-metrics"
  - "/metrics/secret-engine-metrics"
---

# Metrics

Metrics guidance covers collection profiles and the normalization of OpenBao
source metrics into stable derived signals.

## Topics

- [Understanding OpenBao metrics](/metrics/understanding-openbao-metrics/)
  explains metric families, prefixes, and fixture validation.
- [Secure metrics scrape](/metrics/secure-metrics-scrape/) configures
  authenticated active-node scraping.
- [All-node metrics scrape](/metrics/all-node-metrics-scrape/) configures
  private per-node visibility for HA and Raft diagnostics.
- [OpenBao secret engine metrics](/metrics/secret-engine-metrics/) describes
  aggregate and feature-specific secret engine signals.
