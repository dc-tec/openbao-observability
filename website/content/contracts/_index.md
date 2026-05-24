---
title: "Contracts"
description: "Reference material for metric prefixes, recording rules, and source contracts."
weight: 90
browse:
  - "/contracts/metric-prefix"
---

# Contracts

Use this section to understand the source contracts that drive generated
Prometheus, Loki, Grafana, and reference artifacts.

## Topics

- [Metric prefixes and recording rules](/contracts/metric-prefix/) explains the
  `vault_*` and `openbao_*` source-prefix strategy.
- [Metric contracts](https://github.com/dc-tec/openbao-observability/tree/main/contracts/metrics)
  define source metrics and derived recording rules.
- [Dashboard contracts](https://github.com/dc-tec/openbao-observability/tree/main/contracts/dashboards)
  define generated Grafana dashboard intent.
