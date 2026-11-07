package splunk

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

type alertCoverage struct {
	Semantics string          `json:"semantics"`
	Alerts    []alertEvidence `json:"alerts"`
}

type alertEvidence struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Severity    string `json:"severity"`
	HoldSeconds int    `json:"holdSeconds"`
	Earliest    string `json:"earliest"`
	Runbook     string `json:"runbook"`
	Description string `json:"description"`
}

func addAlerts(artifacts map[string][]byte, root string, profile contracts.SplunkProfileContract) error {
	var config strings.Builder
	config.WriteString("# Generated native alerts. Configure scope and an execution owner before enabling.\n")
	mapped := map[string]alertEvidence{}
	for _, binding := range profile.Alerts {
		canonical, err := contracts.SplunkCanonicalAlert(root, binding)
		if err != nil {
			return err
		}
		hold, _ := time.ParseDuration(canonical.For) // The contract loader validates this duration.
		seconds := int(hold.Seconds())
		evidence := alertEvidence{
			ID: canonical.ID, Source: binding.Source, Status: "mapped", Severity: canonical.Severity,
			HoldSeconds: seconds, Earliest: fmt.Sprintf("-%dm", (seconds+binding.WarmupSeconds+59)/60+1),
			Runbook: canonical.Runbook, Description: binding.Description,
		}
		query := alertQuery(profile, binding, canonical, seconds)
		artifacts["alerts/"+canonical.ID+".spl"] = []byte(query + "\n")
		config.WriteString(savedAlert(canonical, evidence, query))
		mapped[canonical.ID] = evidence
	}
	report, err := buildAlertCoverage(root, mapped)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	artifacts["alert-coverage.json"] = append(data, '\n')
	app := "app/" + profile.AppID + "/"
	artifacts[app+"default/savedsearches.conf"] = []byte(config.String())
	artifacts[app+"lookups/openbao_alert_targets.csv"] = []byte("cluster,kubernetes_namespace,scrape_profile,instance\n")
	return nil
}

func alertQuery(
	profile contracts.SplunkProfileContract, binding contracts.SplunkAlertBinding, alert contracts.Alert, hold int,
) string {
	query := strings.ReplaceAll(binding.Search, "${p}", profile.MetricsPrefix)
	query = strings.NewReplacer("${cluster}", strconv.Quote(profile.DefaultCluster),
		"${scrapeProfile}", strconv.Quote(profile.DefaultScrapeProfile)).Replace(query)
	if strings.HasPrefix(query, "`openbao_audit_events`") {
		query = strings.Replace(query, "`openbao_audit_events`", "`openbao_audit_events`\n| search host="+
			strconv.Quote(profile.AlertLogHostPattern)+
			fmt.Sprintf("\n| addinfo | where _time>info_max_time-%d", binding.WarmupSeconds), 1)
		query += "\n| eval cluster=" + strconv.Quote(profile.DefaultCluster) +
			", scrape_profile=" + strconv.Quote(profile.DefaultScrapeProfile)
	} else {
		end := strings.Index(query, "`\n") + 1
		query = query[:end] + "\n| search cluster=" + strconv.Quote(profile.DefaultCluster) +
			" scrape_profile=" + strconv.Quote(profile.DefaultScrapeProfile) + query[end:]
	}
	query += alertHold(hold)
	url := "https://dc-tec.github.io/openbao-observability/" +
		strings.TrimSuffix(strings.TrimPrefix(alert.Runbook, "docs/"), ".md") + "/"
	query += "\n| eval alert_id=" + strconv.Quote(alert.ID) + ", severity=" + strconv.Quote(alert.Severity) +
		", summary=" + strconv.Quote(alert.Summary) + ", runbook_url=" + strconv.Quote(url)
	return strings.TrimSpace(query + "\n| fields alert_id severity summary runbook_url cluster " +
		"kubernetes_namespace scrape_profile instance value")
}

// alertHold evaluates an observed condition history. Gaps older than 30 seconds
// reset persistence; unknown inputs use -1. This is retrospective window evidence,
// not Prometheus's in-memory pending state or a recovery notification engine.
func alertHold(hold int) string {
	return fmt.Sprintf(`
| eval kubernetes_namespace=coalesce(kubernetes_namespace,"(none)"), instance=coalesce(instance,"(cluster)")
| eval condition_at=bucket_time
| eval alert_key=sha256(cluster . "|" . kubernetes_namespace . "|" . scrape_profile . "|" . instance)
| appendpipe [ stats count BY alert_key cluster kubernetes_namespace scrape_profile instance
  | addinfo | eval anchor=floor(info_max_time/15)*15-15, bucket_time=mvrange(anchor-%d,anchor+1,15)
  | mvexpand bucket_time | fields - count ]
| stats max(condition) AS condition max(condition_at) AS condition_at max(value) AS value
  BY bucket_time alert_key cluster kubernetes_namespace scrape_profile instance
| eval _time=bucket_time
| sort 0 alert_key bucket_time
| streamstats last(condition) AS held last(condition_at) AS held_at last(value) AS value BY alert_key
| addinfo | eval anchor=floor(info_max_time/15)*15-15
| where bucket_time>=anchor-%d AND bucket_time<=anchor
| eval satisfied=if(held=1 AND bucket_time-held_at<=30,1,0)
| stats min(satisfied) AS sustained count AS steps latest(value) AS value
  BY cluster kubernetes_namespace scrape_profile instance
| where sustained=1 AND steps>=%d`, hold, hold, hold/15+1)
}

func savedAlert(alert contracts.Alert, evidence alertEvidence, query string) string {
	severity := 3
	if alert.Severity == "critical" {
		severity = 5
	}
	description := strings.NewReplacer("\n", " ", "\r", " ").Replace(alert.Description + " " + evidence.Description)
	return fmt.Sprintf(`
[%s]
description = %s
search = %s
disabled = 1
is_scheduled = 1
cron_schedule = */5 * * * *
dispatch.earliest_time = %s
dispatch.latest_time = -30s
dispatch.ttl = 1h
max_concurrent = 1
alert_type = number of events
alert_comparator = greater than
alert_threshold = 0
alert.track = 1
alert.digest_mode = 0
alert.severity = %d
alert.expires = 1d
alert.suppress = 0
actions =
`, alert.ID, description, strings.ReplaceAll(query, "\n", " "), evidence.Earliest, severity)
}

func buildAlertCoverage(root string, mapped map[string]alertEvidence) (alertCoverage, error) {
	report := alertCoverage{Semantics: "Mapped native searches preserve selected alert intent. " +
		"Persistence uses complete retrospective condition history, with a 30-second sampling-gap tolerance. " +
		"All alerts are disabled by default. Missing inputs reset persistence. Unmapped alerts are future work."}
	paths, err := filepath.Glob(filepath.Join(root, "contracts/alerts/*.yaml"))
	if err != nil {
		return report, err
	}
	for _, path := range paths {
		contract, err := contracts.LoadAlertContract(path)
		if err != nil {
			return report, err
		}
		for _, alert := range contract.Alerts {
			evidence, ok := mapped[alert.ID]
			if !ok {
				evidence = alertEvidence{
					ID: alert.ID, Source: "contracts/alerts/" + filepath.Base(path),
					Status: "unmapped", Severity: alert.Severity, Runbook: alert.Runbook,
					Description: "Outside the focused first native-alert set.",
				}
			}
			report.Alerts = append(report.Alerts, evidence)
		}
	}
	return report, nil
}
