---
title: "Audit"
description: "Audit devices, audit logs, canary checks, archive health, and security-record handling."
weight: 60
browse:
  - "/audit/declarative-audit"
  - "/concepts/audit-logs-as-security-records"
  - "/audit/audit-archive-reference-design"
  - "/dashboards/audit-overview"
  - "/dashboards/audit-investigation"
  - "/runbooks/audit-request-response-failures"
  - "/runbooks/audit-canary-missing"
  - "/runbooks/audit-archive-degraded"
---

# Audit

Audit guidance covers audit devices, restricted log pipelines, archive health,
and audit-related security detections.

## Topics

- [Configure declarative audit devices](/audit/declarative-audit/) explains the
  local and Kubernetes audit-device configuration pattern.
- [Audit logs as security records](/concepts/audit-logs-as-security-records/)
  explains what audit logs represent and why access must stay restricted.
- [Audit archive reference design](/audit/audit-archive-reference-design/)
  explains durable archive responsibilities and health signals.
