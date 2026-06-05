# Generated artifacts

This directory contains generated artifacts produced from contracts. Do not edit
generated files by hand.

Generated artifacts inherit maturity from their source contracts. Check the
top-level `maturity.lifecycle` and `maturity.evidence` fields in `contracts/`
before you treat generated output as stable, reference-only, or draft.

- `prometheus/`: native Prometheus rule files.
- `prometheusrules/`: Prometheus Operator `PrometheusRule` manifests.
- `loki/`: Loki alert reference artifacts.
- `grafana/`: Grafana dashboard JSON files.
- `docs/`: generated reference documents derived from contracts and fixtures.
