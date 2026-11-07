---
title: "Profiles"
description: "Implementation profiles for Prometheus, Loki, Grafana, Alloy, Splunk, Docker Compose, Kubernetes, and the OpenBao Operator."
weight: 30
browse:
  - "/implementation-profiles/prometheus-loki-grafana-alloy"
  - "/implementation-profiles/splunk-enterprise"
  - "/implementation-profiles/splunk-alerts"
  - "/implementation-profiles/splunk-enterprise-validation"
  - "/implementation-profiles/splunk-browser-review"
  - "/docker-compose"
  - "/metrics/secure-metrics-scrape"
  - "/metrics/all-node-metrics-scrape"
  - "/implementation-profiles/openbao-operator"
  - "/implementation-profiles/openbao-operator-integration-contract"
---

# Profiles

Implementation profiles apply the reference architecture to the included stack,
generated artifacts, Kubernetes, and operator-managed OpenBao clusters.

## Topics

- [Prometheus, Loki, Grafana, and Alloy](/implementation-profiles/prometheus-loki-grafana-alloy/)
  explains the tested implementation profile.
- [Splunk Enterprise and Cloud Platform](/implementation-profiles/splunk-enterprise/)
  covers Dashboard Studio dashboards, native searches, collection, and installation.
- [Configure native Splunk alerts](/implementation-profiles/splunk-alerts/)
  describes alert coverage, deployment scope, execution ownership, and activation.
- [Splunk Enterprise validation](/implementation-profiles/splunk-enterprise-validation/)
  records the tested environment, checks, and remaining qualification work.
- [Review the Splunk dashboards in a browser](/implementation-profiles/splunk-browser-review/)
  provides the rendering, filter, and access checklist.
- [Run the Docker Compose stack](/docker-compose/) starts a local OpenBao HA
  environment with dashboards, alerts, and logs.
- [OpenBao Operator companion profile](/implementation-profiles/openbao-operator/)
  explains how this repository pairs with `dc-tec/openbao-operator`.
