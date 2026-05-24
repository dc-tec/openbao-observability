---
title: "Logging"
description: "Operational logs, audit logs, Loki label strategy, retention, and access-control guidance."
weight: 50
browse:
  - "/logging/understanding-openbao-logs"
  - "/logging/loki-label-strategy"
  - "/logging/retention-and-access-control"
  - "/audit/audit-archive-reference-design"
---

# Logging

Use this section to design OpenBao operational log and audit log pipelines
without turning sensitive request metadata into unsafe labels or broad access.

## Topics

- [Understanding OpenBao logs](/logging/understanding-openbao-logs/) explains
  operational logs, completed request logs, and audit logs.
- [Loki label strategy](/logging/loki-label-strategy/) explains safe labels and
  query-time parsing.
- [Log retention and access control](/logging/retention-and-access-control/)
  explains retention boundaries for operational and audit records.
