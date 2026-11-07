package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// SplunkCanonicalAlert resolves the existing alert intent without translating PromQL or LogQL.
func SplunkCanonicalAlert(root string, binding SplunkAlertBinding) (Alert, error) {
	switch binding.Source {
	case "contracts/alerts/critical.yaml", "contracts/alerts/warning.yaml", "contracts/alerts/security.yaml":
	default:
		return Alert{}, fmt.Errorf("invalid canonical Splunk alert source %q", binding.Source)
	}
	contract, err := LoadAlertContract(filepath.Join(root, binding.Source))
	if err != nil {
		return Alert{}, err
	}
	for _, alert := range contract.Alerts {
		if alert.ID == binding.SourceAlert {
			return alert, nil
		}
	}
	return Alert{}, fmt.Errorf("unknown canonical Splunk alert %q", binding.SourceAlert)
}

func validateSplunkAlerts(profile SplunkProfileContract, root string) error {
	if len(profile.Alerts) == 0 {
		return nil
	}
	if profile.AlertLogHostPattern == "" || strings.ContainsAny(profile.AlertLogHostPattern, "\r\n") ||
		strings.ContainsAny(profile.DefaultCluster+profile.DefaultScrapeProfile, "*?\r\n") {
		return fmt.Errorf("native alerts require a host scope and one literal cluster and scrape profile")
	}
	seen := map[string]bool{}
	for _, binding := range profile.Alerts {
		if seen[binding.SourceAlert] {
			return fmt.Errorf("duplicate Splunk alert %q", binding.SourceAlert)
		}
		seen[binding.SourceAlert] = true
		if err := validateSplunkAlert(root, binding); err != nil {
			return err
		}
	}
	return nil
}

func validateSplunkAlert(root string, binding SplunkAlertBinding) error {
	alert, err := SplunkCanonicalAlert(root, binding)
	if err != nil {
		return err
	}
	expressionHash := sha256.Sum256([]byte(alert.Expression))
	if binding.ExpressionSHA != hex.EncodeToString(expressionHash[:]) {
		return fmt.Errorf("native alert %s needs review after a canonical expression change", alert.ID)
	}
	hold, err := time.ParseDuration(alert.For)
	if err != nil || hold < 0 || hold > time.Hour || hold%(15*time.Second) != 0 {
		return fmt.Errorf("unsupported native alert persistence %q", alert.For)
	}
	if binding.Description == "" || binding.WarmupSeconds < 0 || binding.WarmupSeconds > 21600 {
		return fmt.Errorf("invalid Splunk alert description or history window")
	}
	search := strings.TrimSpace(binding.Search)
	if !validSplunkAlertSearch(alert.Type, search) {
		return fmt.Errorf("native alert %s requires a fixed normalized source without dashboard tokens", alert.ID)
	}
	if !strings.Contains(search, "condition") || !strings.Contains(search, "bucket_time") {
		return fmt.Errorf("native alert %s requires condition history", alert.ID)
	}
	return nil
}

func validSplunkAlertSearch(kind, search string) bool {
	if strings.Contains(search, "|s$") || !strings.Contains(search, "`\n") {
		return false
	}
	metricSource := strings.HasPrefix(search, "| `openbao_metric_alert_samples(")
	logSource := strings.HasPrefix(search, "`openbao_audit_events`")
	return kind == "prometheus" && metricSource || kind == "loki" && logSource
}
