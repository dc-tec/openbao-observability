---
title: "OpenBao Observability"
description: "Reference architecture documentation for OpenBao metrics, logs, audit logs, dashboards, alerts, and runbooks."
hero_label: "Reference architecture"
primary_href: "reference-architecture/overview/"
primary_label: "Read architecture"
secondary_href: "docker-compose/"
secondary_label: "Run locally"
hero_image_path: "docs/assets/grafana-dashboards.png"
hero_image_alt: "Grafana dashboard collage showing OpenBao overview, HA/Raft, audit, and feature-specific observability panels."
---

OpenBao Observability defines portable signal, dashboard, alert, and runbook
contracts. The documentation explains how to run the local validation profile
and adapt the generated artifacts to another observability platform.

The architecture starts from verified OpenBao behavior and source contracts,
then publishes a tested Prometheus, Loki, Grafana, and Grafana Alloy profile
that you can adapt to your own monitoring and logging platform.
