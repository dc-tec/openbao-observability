# Tests

This directory contains integration test notes and test fixtures that do not
belong in Go packages. Go unit tests live next to the package they test.

Use `make validate-dashboard-queries` when the Docker Compose stack is running.
That target validates dashboard contract queries and generated Grafana dashboard
queries against the local Prometheus and Loki APIs.
