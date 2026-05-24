package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type options struct {
	ListenAddress string
	MetricsPath   string
	StatusFile    string
	Enabled       bool
	Backend       string
	Pipeline      string
}

type fileStatus struct {
	Enabled                     *bool    `json:"enabled"`
	DeliverySuccess             *bool    `json:"delivery_success"`
	LastSuccessTimestamp        string   `json:"last_success_timestamp"`
	LastSuccessTimestampSeconds *float64 `json:"last_success_timestamp_seconds"`
	DeliveryFailuresTotal       float64  `json:"delivery_failures_total"`
	DeadLetterRecordsTotal      float64  `json:"dead_letter_records_total"`
	Backend                     string   `json:"backend"`
	Pipeline                    string   `json:"pipeline"`
}

type archiveStatus struct {
	Enabled                     bool
	DeliverySuccess             bool
	LastSuccessTimestampSeconds float64
	DeliveryFailuresTotal       float64
	DeadLetterRecordsTotal      float64
	Backend                     string
	Pipeline                    string
}

type archiveCollector struct {
	opts                        options
	enabled                     *prometheus.Desc
	deliverySuccess             *prometheus.Desc
	lastSuccessTimestampSeconds *prometheus.Desc
	deliveryFailuresTotal       *prometheus.Desc
	deadLetterRecordsTotal      *prometheus.Desc
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(newArchiveCollector(opts))

	mux := http.NewServeMux()
	mux.Handle(opts.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	log.Printf("serving audit archive health metrics on %s%s", opts.ListenAddress, opts.MetricsPath)
	server := &http.Server{
		Addr:              opts.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func parseFlags(args []string) (options, error) {
	opts := options{}
	fs := flag.NewFlagSet("audit-archive-health-exporter", flag.ContinueOnError)
	fs.StringVar(&opts.ListenAddress, "listen-address", ":19110", "address for the HTTP server")
	fs.StringVar(&opts.MetricsPath, "metrics-path", "/metrics", "HTTP path for Prometheus metrics")
	fs.StringVar(&opts.StatusFile, "status-file", "archive-health.json", "JSON status file written by the archive pipeline")
	fs.BoolVar(&opts.Enabled, "enabled", false, "mark archive delivery as required when the status file is missing or omits enabled")
	fs.StringVar(&opts.Backend, "backend", "example", "stable backend label")
	fs.StringVar(&opts.Pipeline, "pipeline", "openbao-audit-archive", "stable pipeline label")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if opts.MetricsPath == "" || !strings.HasPrefix(opts.MetricsPath, "/") {
		return options{}, fmt.Errorf("--metrics-path must start with /")
	}
	if opts.StatusFile == "" {
		return options{}, fmt.Errorf("--status-file is required")
	}
	if opts.Backend == "" {
		return options{}, fmt.Errorf("--backend is required")
	}
	if opts.Pipeline == "" {
		return options{}, fmt.Errorf("--pipeline is required")
	}
	return opts, nil
}

func newArchiveCollector(opts options) *archiveCollector {
	labels := []string{"backend", "pipeline"}
	return &archiveCollector{
		opts: opts,
		enabled: prometheus.NewDesc(
			"openbao_audit_archive_enabled",
			"Whether durable OpenBao audit archive delivery is expected for this environment.",
			labels,
			nil,
		),
		deliverySuccess: prometheus.NewDesc(
			"openbao_audit_archive_delivery_success",
			"Whether the OpenBao audit archive delivery path is currently healthy.",
			labels,
			nil,
		),
		lastSuccessTimestampSeconds: prometheus.NewDesc(
			"openbao_audit_archive_last_success_timestamp_seconds",
			"Unix timestamp for the last successful OpenBao audit archive delivery or acknowledgement.",
			labels,
			nil,
		),
		deliveryFailuresTotal: prometheus.NewDesc(
			"openbao_audit_archive_delivery_failures_total",
			"Total failed OpenBao audit archive writes, rejected batches, or failed delivery acknowledgements.",
			labels,
			nil,
		),
		deadLetterRecordsTotal: prometheus.NewDesc(
			"openbao_audit_archive_dead_letter_records_total",
			"Total OpenBao audit records sent to a dead-letter path instead of the durable archive.",
			labels,
			nil,
		),
	}
}

func (c *archiveCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.enabled
	ch <- c.deliverySuccess
	ch <- c.lastSuccessTimestampSeconds
	ch <- c.deliveryFailuresTotal
	ch <- c.deadLetterRecordsTotal
}

func (c *archiveCollector) Collect(ch chan<- prometheus.Metric) {
	status, err := loadStatus(c.opts)
	if err != nil && c.opts.Enabled {
		status.Enabled = true
		status.DeliverySuccess = false
	}

	labelValues := []string{status.Backend, status.Pipeline}
	ch <- prometheus.MustNewConstMetric(c.enabled, prometheus.GaugeValue, boolFloat(status.Enabled), labelValues...)

	if !status.Enabled {
		return
	}

	ch <- prometheus.MustNewConstMetric(c.deliverySuccess, prometheus.GaugeValue, boolFloat(status.DeliverySuccess), labelValues...)
	if status.LastSuccessTimestampSeconds > 0 {
		ch <- prometheus.MustNewConstMetric(c.lastSuccessTimestampSeconds, prometheus.GaugeValue, status.LastSuccessTimestampSeconds, labelValues...)
	}
	ch <- prometheus.MustNewConstMetric(c.deliveryFailuresTotal, prometheus.CounterValue, status.DeliveryFailuresTotal, labelValues...)
	ch <- prometheus.MustNewConstMetric(c.deadLetterRecordsTotal, prometheus.CounterValue, status.DeadLetterRecordsTotal, labelValues...)
}

func loadStatus(opts options) (archiveStatus, error) {
	status := archiveStatus{
		Enabled:  opts.Enabled,
		Backend:  opts.Backend,
		Pipeline: opts.Pipeline,
	}

	content, err := os.ReadFile(opts.StatusFile)
	if err != nil {
		return status, fmt.Errorf("read status file: %w", err)
	}

	var parsed fileStatus
	if err := json.Unmarshal(content, &parsed); err != nil {
		return status, fmt.Errorf("parse status file: %w", err)
	}

	if parsed.Enabled != nil {
		status.Enabled = *parsed.Enabled
	}
	if parsed.DeliverySuccess != nil {
		status.DeliverySuccess = *parsed.DeliverySuccess
	}
	if parsed.Backend != "" {
		status.Backend = parsed.Backend
	}
	if parsed.Pipeline != "" {
		status.Pipeline = parsed.Pipeline
	}

	timestamp, err := parseLastSuccessTimestamp(parsed)
	if err != nil {
		return status, err
	}
	status.LastSuccessTimestampSeconds = timestamp
	status.DeliveryFailuresTotal = parsed.DeliveryFailuresTotal
	status.DeadLetterRecordsTotal = parsed.DeadLetterRecordsTotal

	if err := validateStatus(status); err != nil {
		return status, err
	}
	return status, nil
}

func parseLastSuccessTimestamp(status fileStatus) (float64, error) {
	if status.LastSuccessTimestampSeconds != nil {
		return *status.LastSuccessTimestampSeconds, nil
	}

	value := strings.TrimSpace(status.LastSuccessTimestamp)
	if value == "" {
		return 0, nil
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		return seconds, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0, fmt.Errorf("parse last_success_timestamp: %w", err)
	}
	return float64(parsed.Unix()), nil
}

func validateStatus(status archiveStatus) error {
	if status.Backend == "" {
		return errors.New("backend label cannot be empty")
	}
	if status.Pipeline == "" {
		return errors.New("pipeline label cannot be empty")
	}
	if status.LastSuccessTimestampSeconds < 0 {
		return errors.New("last success timestamp cannot be negative")
	}
	if status.DeliveryFailuresTotal < 0 {
		return errors.New("delivery failures total cannot be negative")
	}
	if status.DeadLetterRecordsTotal < 0 {
		return errors.New("dead-letter records total cannot be negative")
	}
	return nil
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
