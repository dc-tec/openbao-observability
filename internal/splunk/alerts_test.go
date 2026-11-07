package splunk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeAlertsAreScopedDisabledAndAccountForAllCanonicalRules(t *testing.T) {
	artifacts, err := build(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	var report alertCoverage
	if err := json.Unmarshal(artifacts["alert-coverage.json"], &report); err != nil {
		t.Fatal(err)
	}
	mapped := 0
	for _, alert := range report.Alerts {
		if alert.Status != "mapped" {
			continue
		}
		mapped++
		query := string(artifacts["alerts/"+alert.ID+".spl"])
		if strings.Contains(query, "|s$") || !strings.Contains(query, "runbook_url=") ||
			!strings.Contains(query, `cluster="compose"`) || !strings.Contains(query, "| fields alert_id severity") {
			t.Fatal("native alert lacks fixed scope or restricted result fields", alert.ID)
		}
		if alert.ID == "OpenBaoSealedUnexpectedly" && alert.HoldSeconds != 120 {
			t.Fatal("native persistence differs from canonical contract")
		}
	}
	if mapped != 17 || len(report.Alerts) != 49 {
		t.Fatalf("alert coverage: mapped=%d canonical=%d", mapped, len(report.Alerts))
	}
}

func TestNativeAlertsInstallDisabledWithoutInventedTargets(t *testing.T) {
	artifacts, err := build(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	config := string(artifacts["app/openbao_observability/default/savedsearches.conf"])
	if strings.Count(config, "disabled = 1\n") != 17 || strings.Count(config, "\nactions =\n") != 17 {
		t.Fatal("native alerts can schedule or send outbound actions at installation")
	}
	if string(artifacts["app/openbao_observability/lookups/openbao_alert_targets.csv"]) !=
		"cluster,kubernetes_namespace,scrape_profile,instance\n" {
		t.Fatal("release artifact must not invent expected targets")
	}
}
