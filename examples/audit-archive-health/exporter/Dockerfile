FROM golang:1.26.6-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY examples/audit-archive-health/exporter ./examples/audit-archive-health/exporter
RUN CGO_ENABLED=0 go build -o /out/audit-archive-health-exporter ./examples/audit-archive-health/exporter

FROM scratch

USER 65532:65532
COPY --from=build /out/audit-archive-health-exporter /audit-archive-health-exporter

ENTRYPOINT ["/audit-archive-health-exporter"]
