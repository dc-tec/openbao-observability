package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStatusParsesStatusFile(t *testing.T) {
	path := writeStatusFile(t, `{
  "enabled": true,
  "delivery_success": true,
  "last_success_timestamp": "2026-05-24T12:00:00Z",
  "delivery_failures_total": 2,
  "dead_letter_records_total": 3,
  "backend": "siem",
  "pipeline": "openbao-audit"
}`)

	status, err := loadStatus(options{
		StatusFile: path,
		Backend:    "default-backend",
		Pipeline:   "default-pipeline",
	})
	if err != nil {
		t.Fatalf("loadStatus returned error: %v", err)
	}

	if !status.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if !status.DeliverySuccess {
		t.Fatal("DeliverySuccess = false, want true")
	}
	if status.LastSuccessTimestampSeconds != 1779624000 {
		t.Fatalf("LastSuccessTimestampSeconds = %v, want 1779624000", status.LastSuccessTimestampSeconds)
	}
	if status.DeliveryFailuresTotal != 2 {
		t.Fatalf("DeliveryFailuresTotal = %v, want 2", status.DeliveryFailuresTotal)
	}
	if status.DeadLetterRecordsTotal != 3 {
		t.Fatalf("DeadLetterRecordsTotal = %v, want 3", status.DeadLetterRecordsTotal)
	}
	if status.Backend != "siem" {
		t.Fatalf("Backend = %q, want siem", status.Backend)
	}
	if status.Pipeline != "openbao-audit" {
		t.Fatalf("Pipeline = %q, want openbao-audit", status.Pipeline)
	}
}

func TestLoadStatusUsesEnabledFlagWhenFileIsMissing(t *testing.T) {
	status, err := loadStatus(options{
		Enabled:    true,
		StatusFile: filepath.Join(t.TempDir(), "missing.json"),
		Backend:    "object-store",
		Pipeline:   "openbao-audit",
	})
	if err == nil {
		t.Fatal("loadStatus returned nil error for missing status file")
	}
	if !status.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if status.DeliverySuccess {
		t.Fatal("DeliverySuccess = true, want false")
	}
}

func TestLoadStatusRejectsNegativeCounters(t *testing.T) {
	path := writeStatusFile(t, `{
  "enabled": true,
  "delivery_success": false,
  "delivery_failures_total": -1
}`)

	_, err := loadStatus(options{
		StatusFile: path,
		Backend:    "object-store",
		Pipeline:   "openbao-audit",
	})
	if err == nil {
		t.Fatal("loadStatus returned nil error for negative counter")
	}
}

func TestParseFlagsRejectsRelativeMetricsPath(t *testing.T) {
	_, err := parseFlags([]string{"--metrics-path", "metrics"})
	if err == nil {
		t.Fatal("parseFlags returned nil error for relative metrics path")
	}
}

func writeStatusFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write status file: %v", err)
	}
	return path
}
