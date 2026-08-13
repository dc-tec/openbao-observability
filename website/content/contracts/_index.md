---
title: "Contracts"
description: "Reference material for metric prefixes, recording rules, and source contracts."
weight: 90
browse:
  - "/contracts/metric-prefix"
---

# Contracts

Source contracts define the Prometheus, Loki, Grafana, and reference artifacts
that this repository generates.

## Topics

- [Metric prefixes and recording rules](/contracts/metric-prefix/) explains the
  `vault_*` and `openbao_*` source-prefix strategy.
- [Metric contracts](https://github.com/dc-tec/openbao-observability/tree/main/contracts/metrics)
  define source metrics and derived recording rules.
- [Dashboard contracts](https://github.com/dc-tec/openbao-observability/tree/main/contracts/dashboards)
  define generated Grafana dashboard intent.
