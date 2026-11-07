package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SplunkProfileContract binds existing dashboard questions to native SPL.
// It deliberately does not translate PromQL or LogQL expressions.
type SplunkProfileContract struct {
	Version               string                   `yaml:"version"`
	Maturity              Maturity                 `yaml:"maturity"`
	AppID                 string                   `yaml:"appID"`
	Title                 string                   `yaml:"title"`
	AuditIndex            string                   `yaml:"auditIndex"`
	MetricsIndex          string                   `yaml:"metricsIndex"`
	MetricsPrefix         string                   `yaml:"metricsPrefix"`
	DefaultCluster        string                   `yaml:"defaultCluster"`
	DefaultScrapeProfile  string                   `yaml:"defaultScrapeProfile"`
	Macros                []SplunkMacroBinding     `yaml:"macros"`
	AuditSourcetype       string                   `yaml:"auditSourcetype"`
	OperationalIndex      string                   `yaml:"operationalIndex"`
	OperationalSourcetype string                   `yaml:"operationalSourcetype"`
	CompletedIndex        string                   `yaml:"completedIndex"`
	CompletedSourcetype   string                   `yaml:"completedSourcetype"`
	PlatformIndex         string                   `yaml:"platformIndex"`
	PlatformSourcetype    string                   `yaml:"platformSourcetype"`
	Fields                map[string]string        `yaml:"fields"`
	Dashboards            []SplunkDashboardBinding `yaml:"dashboards"`
	AlertLogHostPattern   string                   `yaml:"alertLogHostPattern"`
	Alerts                []SplunkAlertBinding     `yaml:"alerts"`
}

// SplunkAlertBinding supplies a native condition history for a canonical alert.
// Severity, runbook, and persistence duration come from the canonical contract.
type SplunkAlertBinding struct {
	Source        string `yaml:"source"`
	SourceAlert   string `yaml:"sourceAlert"`
	ExpressionSHA string `yaml:"sourceExpressionSHA256"`
	Search        string `yaml:"search"`
	WarmupSeconds int    `yaml:"warmupSeconds"`
	Description   string `yaml:"description"`
}

type SplunkDashboardBinding struct {
	ID          string               `yaml:"id"`
	Source      string               `yaml:"source"`
	Description string               `yaml:"description"`
	Panels      []SplunkPanelBinding `yaml:"panels"`
	Inputs      []SplunkInputBinding `yaml:"inputs,omitempty"`
}

type SplunkInputBinding struct {
	Name    string `yaml:"name"`
	Default string `yaml:"default"`
}

type SplunkPanelBinding struct {
	SourcePanel string `yaml:"sourcePanel"`
	Description string `yaml:"description"`
	Search      string `yaml:"search"`
	Scope       string `yaml:"scope,omitempty"`
	Earliest    string `yaml:"earliest,omitempty"`
}

type SplunkMacroBinding struct {
	Name       string   `yaml:"name"`
	Arguments  []string `yaml:"arguments"`
	Definition string   `yaml:"definition"`
}

var (
	splunkIDPattern         = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	splunkIndexPattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	splunkSourcetypePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_:.-]*$`)
	splunkJSONPathPattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)
)

func LoadSplunkProfile(path, root string) (*SplunkProfileContract, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- The CLI selects the source contract path.
	if err != nil {
		return nil, err
	}
	var profile SplunkProfileContract
	if err := decodeContractYAML(data, &profile); err != nil {
		return nil, fmt.Errorf("parse Splunk profile: %w", err)
	}
	if err := validateContractVersion(path, profile.Version); err != nil {
		return nil, err
	}
	if err := validateMaturity(path, profile.Maturity); err != nil {
		return nil, err
	}
	if err := validateSplunkSource(profile); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, dashboard := range profile.Dashboards {
		if !splunkIDPattern.MatchString(dashboard.ID) || seen[dashboard.ID] {
			return nil, fmt.Errorf("invalid or duplicate Splunk dashboard ID %q", dashboard.ID)
		}
		seen[dashboard.ID] = true
		if err := validateSplunkDashboard(root, dashboard); err != nil {
			return nil, err
		}
	}
	if err := validateSplunkReferences(profile); err != nil {
		return nil, err
	}
	if err := validateSplunkAlerts(profile, root); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SplunkAuditFields is the allowlist exposed by the normalized audit macro.
// Credentials, token accessors, request data, and response data are excluded.
func SplunkAuditFields() []string {
	return []string{
		"audit_type", "request_id", "request_path", "operation",
		"logical_namespace", "caller", "client_ip", "audit_error",
	}
}

func validateSplunkDashboard(root string, binding SplunkDashboardBinding) error {
	source := filepath.ToSlash(filepath.Clean(binding.Source))
	if source != binding.Source || filepath.ToSlash(filepath.Dir(source)) != "contracts/dashboards" ||
		filepath.Ext(source) != ".yaml" {
		return fmt.Errorf("the Splunk dashboard source must reference contracts/dashboards/*.yaml")
	}
	dashboard, err := LoadDashboardContract(filepath.Join(root, source))
	if err != nil {
		return err
	}
	if strings.TrimSpace(binding.Description) == "" {
		return fmt.Errorf("the Splunk dashboard %s requires a description", binding.ID)
	}
	if err := validateSplunkInputs(binding.Inputs); err != nil {
		return err
	}
	panels := map[string]DashboardPanel{}
	for _, panel := range dashboard.Panels {
		panels[panel.ID] = panel
	}
	seen := map[string]bool{}
	if len(binding.Panels) == 0 {
		return fmt.Errorf("the Splunk dashboard %s has no panels", binding.ID)
	}
	for _, panel := range binding.Panels {
		canonical, ok := panels[panel.SourcePanel]
		if !ok || seen[panel.SourcePanel] {
			return fmt.Errorf("invalid or duplicate source panel %q", panel.SourcePanel)
		}
		seen[panel.SourcePanel] = true
		if err := validateSplunkPanel(canonical, panel); err != nil {
			return err
		}
		if panel.Description == "" || !validSplunkSearch(canonical.Signal, panel.Search) {
			return fmt.Errorf("the Splunk panel %s requires a description and a signal-specific source macro", panel.SourcePanel)
		}
	}
	return nil
}

func validateSplunkInputs(inputs []SplunkInputBinding) error {
	seen := map[string]bool{}
	for _, input := range inputs {
		if !splunkIDPattern.MatchString(input.Name) || seen[input.Name] || input.Default == "" {
			return fmt.Errorf("invalid or duplicate Splunk input %q", input.Name)
		}
		seen[input.Name] = true
	}
	return nil
}

func validateSplunkPanel(canonical DashboardPanel, panel SplunkPanelBinding) error {
	if panel.Scope != "" && panel.Scope != "platform" && panel.Scope != "probe" && panel.Scope != "archive" {
		return fmt.Errorf("invalid Splunk panel scope %q", panel.Scope)
	}
	if panel.Scope != "" && canonical.Signal != "metrics" {
		return fmt.Errorf("external metric scope requires a metric panel")
	}
	if panel.Earliest != "" && (!regexp.MustCompile(`^-[1-9][0-9]*(m|h|d)$`).MatchString(panel.Earliest) ||
		canonical.Type != "stat") {
		return fmt.Errorf("invalid Splunk panel lookback %q", panel.Earliest)
	}
	switch canonical.Type {
	case "stat", "timeseries", "logs":
		return nil
	default:
		return fmt.Errorf("unsupported Splunk source panel type %q", canonical.Type)
	}
}

func validSplunkSearch(signal, search string) bool {
	search = strings.TrimSpace(search)
	if signal == "logs" {
		return strings.HasPrefix(search, "`openbao_audit_events`") ||
			strings.HasPrefix(search, "`openbao_operational_events`") ||
			strings.HasPrefix(search, "`openbao_completed_events`") ||
			strings.HasPrefix(search, "`openbao_platform_events`")
	}
	return signal == "metrics" && strings.HasPrefix(search, "| `openbao_metric_samples(")
}

func validateSplunkSource(profile SplunkProfileContract) error {
	if !splunkIDPattern.MatchString(profile.AppID) || strings.TrimSpace(profile.Title) == "" ||
		strings.ContainsAny(profile.Title, "\r\n") {
		return fmt.Errorf("the Splunk profile requires a safe appID and title")
	}
	if err := validateSplunkIndexes(profile); err != nil {
		return err
	}
	for _, name := range SplunkAuditFields() {
		if !splunkJSONPathPattern.MatchString(profile.Fields[name]) {
			return fmt.Errorf("invalid or missing Splunk audit field %s", name)
		}
	}
	if len(profile.Fields) != len(SplunkAuditFields()) {
		return fmt.Errorf("the Splunk profile contains unsupported audit fields")
	}
	if profile.MetricsPrefix != "vault" && profile.MetricsPrefix != "openbao" {
		return fmt.Errorf("invalid Splunk metrics prefix")
	}
	if err := validateSplunkMacros(profile.Macros); err != nil {
		return err
	}
	if len(profile.Dashboards) == 0 {
		return fmt.Errorf("the Splunk profile has no dashboards")
	}

	return nil
}

func validateSplunkIndexes(profile SplunkProfileContract) error {
	for _, source := range []struct{ index, sourcetype string }{
		{profile.AuditIndex, profile.AuditSourcetype},
		{profile.OperationalIndex, profile.OperationalSourcetype},
		{profile.CompletedIndex, profile.CompletedSourcetype},
		{profile.PlatformIndex, profile.PlatformSourcetype},
	} {
		if !splunkIndexPattern.MatchString(source.index) || !splunkSourcetypePattern.MatchString(source.sourcetype) {
			return fmt.Errorf("the Splunk profile requires literal index and sourcetype names")
		}
	}
	if !splunkIndexPattern.MatchString(profile.MetricsIndex) {
		return fmt.Errorf("invalid Splunk metrics index")
	}
	return nil
}

func validateSplunkMacros(macros []SplunkMacroBinding) error {
	seen := map[string]bool{"openbao_metric_source": true}
	for _, macro := range macros {
		if !strings.HasPrefix(macro.Name, "openbao_metric_") || !splunkIDPattern.MatchString(macro.Name) || seen[macro.Name] {
			return fmt.Errorf("invalid or duplicate Splunk metric macro %q", macro.Name)
		}
		seen[macro.Name] = true
		if strings.TrimSpace(macro.Definition) == "" {
			return fmt.Errorf("empty Splunk metric macro %s", macro.Name)
		}
		for _, arg := range macro.Arguments {
			if !splunkIDPattern.MatchString(arg) {
				return fmt.Errorf("invalid macro argument %q", arg)
			}
		}
	}
	return nil
}

func validateSplunkReferences(profile SplunkProfileContract) error {
	known := map[string]bool{"openbao_metric_source": true}
	for _, macro := range profile.Macros {
		known[macro.Name] = true
	}
	reference := regexp.MustCompile("`(openbao_metric_[a-z0-9_]+)")
	searches := make([]string, 0, len(profile.Macros))
	for _, macro := range profile.Macros {
		searches = append(searches, macro.Definition)
	}
	for _, dashboard := range profile.Dashboards {
		for _, panel := range dashboard.Panels {
			searches = append(searches, panel.Search)
		}
	}
	for _, alert := range profile.Alerts {
		searches = append(searches, alert.Search)
	}
	for _, search := range searches {
		for _, match := range reference.FindAllStringSubmatch(search, -1) {
			if !known[match[1]] {
				return fmt.Errorf("unknown Splunk metric macro %s", match[1])
			}
		}
	}
	return nil
}
