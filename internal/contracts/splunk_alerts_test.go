package contracts

import (
	"path/filepath"
	"testing"
)

func TestNativeAlertBindingsRejectScopeAndSourceErrors(t *testing.T) {
	root := filepath.Join("..", "..")
	profile, err := LoadSplunkProfile(filepath.Join(root, "contracts/profiles/splunk-enterprise.yaml"), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*SplunkProfileContract)
	}{
		{"source traversal", func(p *SplunkProfileContract) { p.Alerts[0].Source = "../alerts.yaml" }},
		{"unknown rule", func(p *SplunkProfileContract) { p.Alerts[0].SourceAlert = "Unknown" }},
		{"unreviewed expression", func(p *SplunkProfileContract) { p.Alerts[0].ExpressionSHA = "stale" }},
		{"wrong signal", func(p *SplunkProfileContract) {
			p.Alerts[0].Search = "`openbao_audit_events`\n| eval condition=1, bucket_time=_time"
		}},
		{"dashboard token", func(p *SplunkProfileContract) { p.Alerts[0].Search += "\n| search cluster=$cluster|s$" }},
		{"negative history", func(p *SplunkProfileContract) { p.Alerts[0].WarmupSeconds = -1 }},
		{"duplicate rule", func(p *SplunkProfileContract) { p.Alerts = append(p.Alerts, p.Alerts[0]) }},
		{"ambiguous cluster", func(p *SplunkProfileContract) { p.DefaultCluster = "*" }},
		{"host newline", func(p *SplunkProfileContract) { p.AlertLogHostPattern = "*\nindex=*" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			candidate := *profile
			candidate.Alerts = append([]SplunkAlertBinding(nil), profile.Alerts...)
			change.edit(&candidate)
			if validateSplunkAlerts(candidate, root) == nil {
				t.Fatal("invalid native alert binding accepted")
			}
		})
	}
}
