// Package splunk generates native dashboard bindings for Splunk Enterprise and Cloud Platform.
package splunk

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

const auditMacro = "`openbao_audit_events`"

type Options struct {
	ContractPath   string
	RepositoryRoot string
	OutputDir      string
}

type studioDashboard struct {
	Title          string                         `json:"title"`
	Description    string                         `json:"description"`
	Inputs         map[string]studioInput         `json:"inputs"`
	DataSources    map[string]studioSearch        `json:"dataSources"`
	Visualizations map[string]studioVisualization `json:"visualizations"`
	Layout         studioLayout                   `json:"layout"`
}
type studioInput struct {
	Type    string       `json:"type"`
	Title   string       `json:"title"`
	Options inputOptions `json:"options"`
}
type inputOptions struct {
	Token        string `json:"token"`
	DefaultValue string `json:"defaultValue"`
}
type studioSearch struct {
	Type    string        `json:"type"`
	Name    string        `json:"name"`
	Options searchOptions `json:"options"`
}
type searchOptions struct {
	Query           string         `json:"query"`
	QueryParameters timeParameters `json:"queryParameters"`
}
type timeParameters struct {
	Earliest string `json:"earliest"`
	Latest   string `json:"latest"`
}
type studioVisualization struct {
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	DataSources primarySource  `json:"dataSources"`
	Options     *visualOptions `json:"options,omitempty"`
}
type visualOptions struct {
	Unit           string `json:"unit,omitempty"`
	UnitPosition   string `json:"unitPosition,omitempty"`
	YAxisTitleText string `json:"yAxisTitleText,omitempty"`
	MajorValue     string `json:"majorValue,omitempty"`
}

type (
	primarySource struct {
		Primary string `json:"primary"`
	}
	studioLayout struct {
		GlobalInputs      []string                    `json:"globalInputs"`
		LayoutDefinitions map[string]layoutDefinition `json:"layoutDefinitions"`
		Tabs              layoutTabs                  `json:"tabs"`
	}
)

type (
	layoutTabs struct {
		Items []layoutTab `json:"items"`
	}
	layoutTab struct {
		Label    string `json:"label"`
		LayoutID string `json:"layoutId"`
	}
)

type layoutDefinition struct {
	Type      string        `json:"type"`
	Options   layoutOptions `json:"options"`
	Structure []layoutItem  `json:"structure"`
}
type layoutOptions struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Display string `json:"display"`
}
type layoutItem struct {
	Item     string   `json:"item"`
	Type     string   `json:"type"`
	Position position `json:"position"`
}
type (
	position struct {
		X int `json:"x"`
		Y int `json:"y"`
		W int `json:"w"`
		H int `json:"h"`
	}
	studioView struct {
		XMLName     xml.Name `xml:"dashboard"`
		Version     string   `xml:"version,attr"`
		Label       string   `xml:"label"`
		Description string   `xml:"description"`
		Definition  cdata    `xml:"definition"`
	}
)

type cdata struct {
	Value string `xml:",cdata"`
}

type coverage struct {
	Profile    string              `json:"profile"`
	Status     string              `json:"status"`
	Validation string              `json:"validation"`
	Semantics  string              `json:"semantics"`
	Dashboards []dashboardCoverage `json:"dashboards"`
}
type dashboardCoverage struct {
	Source            string   `json:"source"`
	Status            string   `json:"status"`
	IncludedPanels    []string `json:"includedPanels"`
	UnavailablePanels []string `json:"unavailablePanels"`
	RequiredSources   []string `json:"requiredSources"`
}

// Generate writes only artifacts owned by this profile. Verify detects stale extra files.
func Generate(opts Options) error {
	artifacts, err := build(opts)
	if err != nil {
		return err
	}
	for name, data := range artifacts {
		target := filepath.Join(opts.OutputDir, name)
		// Generated artifacts are public repository files under a caller-selected directory.
		// #nosec G301 -- Generated public artifacts must be readable by repository consumers.
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		// #nosec G306 -- Generated public artifacts contain no credentials.
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Verify checks source bindings and byte-for-byte generated parity, including orphan artifacts.
func Verify(opts Options) error {
	artifacts, err := build(opts)
	if err != nil {
		return err
	}
	for name, expected := range artifacts {
		// #nosec G304 -- The caller selects the generated artifact directory.
		actual, err := os.ReadFile(filepath.Join(opts.OutputDir, name))
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("stale Splunk artifact %s; run make generate-splunk", name)
		}
	}
	return filepath.WalkDir(opts.OutputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name, err := filepath.Rel(opts.OutputDir, path)
		if err != nil {
			return err
		}
		if _, ok := artifacts[filepath.ToSlash(name)]; !ok {
			return fmt.Errorf("unowned Splunk artifact %s", name)
		}
		return nil
	})
}

func build(opts Options) (map[string][]byte, error) {
	profile, err := contracts.LoadSplunkProfile(opts.ContractPath, opts.RepositoryRoot)
	if err != nil {
		return nil, err
	}
	artifacts := map[string][]byte{}
	app := "app/" + profile.AppID + "/"
	artifacts[app+"default/app.conf"] = []byte("# Generated; edit the Splunk profile contract.\n" +
		"[install]\nis_configured = 0\n[ui]\nis_visible = 1\nlabel = " + profile.Title +
		"\n[launcher]\nauthor = OpenBao observability contributors\ndescription = Draft OpenBao observability profile" +
		"\nversion = 0.0.0-dev\n[id]\nname = " + profile.AppID +
		"\nversion = 0.0.0-dev\n[package]\ncheck_for_updates = false\nid = " + profile.AppID + "\n")
	artifacts[app+"metadata/default.meta"] = []byte("[]\n" +
		"access = read : [ admin, sc_admin ], write : [ admin, sc_admin ]\nexport = none\n")
	artifacts[app+"default/macros.conf"] = []byte(macros(*profile))
	if err := addAlerts(artifacts, opts.RepositoryRoot, *profile); err != nil {
		return nil, err
	}
	var nav strings.Builder
	nav.WriteString("<nav>\n")
	for i, binding := range profile.Dashboards {
		attribute := ""
		if i == 0 {
			attribute = ` default="true"`
		}
		fmt.Fprintf(&nav, "  <view name=%q%s/>\n", binding.ID, attribute)
	}
	nav.WriteString("</nav>\n")
	artifacts[app+"default/data/ui/nav/default.xml"] = []byte(nav.String())
	for _, binding := range profile.Dashboards {
		if err := addDashboard(artifacts, app, opts.RepositoryRoot, binding, *profile); err != nil {
			return nil, err
		}
	}
	report, err := buildCoverage(opts.RepositoryRoot, *profile)
	if err != nil {
		return nil, err
	}
	data, err := marshalJSON(report)
	if err != nil {
		return nil, err
	}
	artifacts["coverage.json"] = data
	return artifacts, nil
}

func addDashboard(
	artifacts map[string][]byte, app, root string, binding contracts.SplunkDashboardBinding,
	profile contracts.SplunkProfileContract,
) error {
	canonical, err := contracts.LoadDashboardContract(filepath.Join(root, binding.Source))
	if err != nil {
		return err
	}
	document := buildDashboard(*canonical, binding, profile)
	data, err := marshalJSON(document)
	if err != nil {
		return err
	}
	artifacts["dashboards/"+binding.ID+".json"] = data
	view := studioView{
		Version: "2", Label: document.Title,
		Description: binding.Description, Definition: cdata{string(data)},
	}
	wrapper, err := xml.MarshalIndent(view, "", "  ")
	if err != nil {
		return err
	}
	artifacts[app+"default/data/ui/views/"+binding.ID+".xml"] = append(wrapper, '\n')
	for _, panel := range binding.Panels {
		search := strings.ReplaceAll(panel.Search, "${p}", profile.MetricsPrefix)
		artifacts["searches/"+binding.ID+"/"+panel.SourcePanel+".spl"] = []byte(strings.TrimSpace(search) + "\n")
	}
	return nil
}

func buildDashboard(
	canonical contracts.DashboardContract, binding contracts.SplunkDashboardBinding,
	profile contracts.SplunkProfileContract,
) studioDashboard {
	inputs := map[string]studioInput{
		"input_time": {Type: "input.timerange", Title: "Time range", Options: inputOptions{"global_time", "-1h@h,now"}},
	}
	globalInputs := make([]string, 1, 8)
	globalInputs[0] = "input_time"
	for _, name := range dashboardInputNames(canonical, binding) {
		id := "input_" + name
		defaultValue := "*"
		if name == "cluster" && profile.DefaultCluster != "" {
			defaultValue = profile.DefaultCluster
		}
		if name == "scrape_profile" && profile.DefaultScrapeProfile != "" {
			defaultValue = profile.DefaultScrapeProfile
		}
		for _, input := range binding.Inputs {
			if input.Name == name {
				defaultValue = input.Default
			}
		}
		inputs[id] = studioInput{
			Type: "input.text", Title: inputTitle(name),
			Options: inputOptions{name, defaultValue},
		}
		globalInputs = append(globalInputs, id)
	}
	document := studioDashboard{
		Title: canonical.Title, Description: binding.Description, Inputs: inputs,
		DataSources: map[string]studioSearch{}, Visualizations: map[string]studioVisualization{},
	}
	panels := map[string]contracts.DashboardPanel{}
	for _, panel := range canonical.Panels {
		panels[panel.ID] = panel
	}
	structure := make([]layoutItem, 0, len(binding.Panels))
	y, x, rowHeight := 0, 0, 0
	for _, binding := range binding.Panels {
		panel := panels[binding.SourcePanel]
		id := strings.ReplaceAll(panel.ID, "-", "_")
		document.DataSources["ds_"+id] = studioSearch{
			Type: "ds.search", Name: panel.Title,
			Options: searchOptions{
				Query:           dashboardQuery(binding, profile),
				QueryParameters: dashboardTime(binding),
			},
		}
		kind, width, height := visualizationShape(panel.Type)
		if x+width > 1200 {
			x = 0
			y += rowHeight + 20
			rowHeight = 0
		}
		document.Visualizations["viz_"+id] = studioVisualization{
			Type: kind, Title: panel.Title,
			Description: binding.Description, DataSources: primarySource{"ds_" + id},
			Options: panelPresentation(panel),
		}
		structure = append(structure, layoutItem{Item: "viz_" + id, Type: "block", Position: position{x, y, width, height}})
		x += width
		rowHeight = max(rowHeight, height)
		if x == 1200 {
			x = 0
			y += rowHeight + 20
			rowHeight = 0
		}
	}
	document.Layout = studioLayout{
		GlobalInputs: globalInputs,
		LayoutDefinitions: map[string]layoutDefinition{"layout_audit": {
			Type:    "absolute",
			Options: layoutOptions{1200, y + rowHeight + 20, "auto-scale"}, Structure: structure,
		}},
		Tabs: layoutTabs{[]layoutTab{{Label: canonical.Title, LayoutID: "layout_audit"}}},
	}
	return document
}

func inputTitle(name string) string {
	if name == "slo_target" {
		return "SLO objective (0 < value < 1)"
	}
	return strings.ReplaceAll(name, "_", " ") + " (wildcard)"
}

func dashboardTime(binding contracts.SplunkPanelBinding) timeParameters {
	if binding.Earliest != "" {
		return timeParameters{binding.Earliest, "now"}
	}
	return timeParameters{"$global_time.earliest$", "$global_time.latest$"}
}

func dashboardQuery(binding contracts.SplunkPanelBinding, profile contracts.SplunkProfileContract) string {
	query := strings.ReplaceAll(binding.Search, "${p}", profile.MetricsPrefix)
	if strings.HasPrefix(query, auditMacro) {
		return strings.TrimSpace(strings.Replace(query, auditMacro, auditMacro+dashboardFilters(), 1))
	}
	if strings.HasPrefix(query, "`openbao_operational_events`") {
		return strings.TrimSpace(strings.Replace(query, "`openbao_operational_events`",
			"`openbao_operational_events`\n| search host=$host|s$", 1))
	}
	if strings.HasPrefix(query, "`openbao_completed_events`") {
		return strings.TrimSpace(strings.Replace(query, "`openbao_completed_events`",
			"`openbao_completed_events`\n| search host=$host|s$", 1))
	}
	if strings.HasPrefix(query, "`openbao_platform_events`") {
		return strings.TrimSpace(strings.Replace(query, "`openbao_platform_events`",
			"`openbao_platform_events`\n| search cluster=$cluster|s$"+
				" kubernetes_namespace=$kubernetes_namespace|s$ pod=$pod|s$ host=$host|s$", 1))
	}
	if end := strings.Index(query[3:], "`"); end >= 0 {
		end += 4
		filters := "\n| search cluster=$cluster|s$ instance=$instance|s$"
		if binding.Scope == "" {
			filters += " scrape_profile=$scrape_profile|s$"
		}
		filters += " kubernetes_namespace=$kubernetes_namespace|s$ openbao_namespace=$openbao_namespace|s$"
		query = query[:end] + filters + query[end:]
	}
	return strings.TrimSpace(query)
}

func visualizationShape(kind string) (string, int, int) {
	switch kind {
	case "stat":
		return "splunk.singlevalue", 300, 120
	case "timeseries":
		return "splunk.line", 600, 260
	default:
		return "splunk.table", 1200, 300
	}
}

func dashboardFilters() string {
	return "\n| search request_id=$request_id|s$ request_path=$request_path|s$" +
		" logical_namespace=$logical_namespace|s$ operation=$operation|s$ host=$host|s$"
}

// normalization uses explicit paths, including fields beyond spath's automatic extraction window.
func normalization(profile contracts.SplunkProfileContract) string {
	var search strings.Builder
	search.WriteString(" | fields _raw _time host")
	for _, name := range contracts.SplunkAuditFields() {
		fmt.Fprintf(&search, " | spath input=_raw path=%s output=%s", profile.Fields[name], name)
	}
	search.WriteString(` | eval logical_namespace=coalesce(logical_namespace,"(root)"),` +
		` caller=coalesce(caller,"(missing)"), client_ip=coalesce(client_ip,"(missing)"),` +
		` request_id=coalesce(request_id,"(missing)"), request_path=coalesce(request_path,"(missing)"),` +
		` operation=coalesce(operation,"(missing)"), audit_type=coalesce(audit_type,"(missing)"),` +
		` audit_error=coalesce(audit_error,"")`)
	search.WriteString(" | fields _time host " + strings.Join(contracts.SplunkAuditFields(), " ") + " | fields - _raw")
	return search.String()
}

// Operational messages can contain sensitive error text. Project only the documented JSON fields.
func operationalNormalization() string {
	return ` | fields _raw _time host` +
		` | spath input=_raw path=@level output=severity` +
		` | spath input=_raw path=@module output=module` +
		` | spath input=_raw path=@message output=message` +
		` | eval severity=lower(coalesce(severity,"(missing)")),` +
		` module=coalesce(module,"(missing)"), message=coalesce(message,"(unparsed JSON)")` +
		` | fields _time host severity module message | fields - _raw`
}

func macros(profile contracts.SplunkProfileContract) string {
	result := "# Generated; use local/macros.conf for installation overrides.\n" +
		"[openbao_audit_source]\ndefinition = index=\"" + profile.AuditIndex + "\" sourcetype=\"" +
		profile.AuditSourcetype + "\"\niseval = 0\n\n[openbao_audit_events]\ndefinition = `openbao_audit_source`" +
		normalization(profile) + "\niseval = 0\n"
	result += "\n[openbao_metric_source]\ndefinition = index=\"" + profile.MetricsIndex + "\"\niseval = 0\n"
	result += "\n[openbao_operational_source]\ndefinition = index=\"" + profile.OperationalIndex +
		"\" sourcetype=\"" + profile.OperationalSourcetype + "\"\niseval = 0\n"
	result += "\n[openbao_operational_events]\ndefinition = `openbao_operational_source`" +
		operationalNormalization() + "\niseval = 0\n"
	result += extraLogMacros(profile)
	for _, macro := range profile.Macros {
		name := macro.Name
		if len(macro.Arguments) > 0 {
			name += fmt.Sprintf("(%d)", len(macro.Arguments))
		}
		definition := strings.Join(strings.Fields(macro.Definition), " ")
		result += "\n[" + name + "]\ndefinition = " + definition + "\niseval = 0\n"
		if len(macro.Arguments) > 0 {
			result += "args = " + strings.Join(macro.Arguments, ",") + "\n"
		}
	}
	return result
}

func extraLogMacros(profile contracts.SplunkProfileContract) string {
	return "\n[openbao_completed_source]\ndefinition = index=\"" + profile.CompletedIndex +
		"\" sourcetype=\"" + profile.CompletedSourcetype + "\"\niseval = 0\n" +
		"\n[openbao_completed_events]\ndefinition = `openbao_completed_source`" + completedNormalization() +
		"\niseval = 0\n\n[openbao_platform_source]\ndefinition = index=\"" + profile.PlatformIndex +
		"\" sourcetype=\"" + profile.PlatformSourcetype + "\"\niseval = 0\n" +
		"\n[openbao_platform_events]\ndefinition = `openbao_platform_source`" + platformNormalization() +
		"\niseval = 0\n"
}

func completedNormalization() string {
	return ` | fields _raw _time host | spath input=_raw path=@message output=event_kind` +
		` | where event_kind="completed_request" | fields _time host event_kind | fields - _raw`
}

func platformNormalization() string {
	return ` | fields _raw _time host cluster` +
		` | spath input=_raw path=cluster output=deployment_cluster` +
		` | spath input=_raw path=object output=event_json` +
		` | eval event_json=coalesce(event_json,_raw)` +
		` | spath input=event_json path=type output=event_type` +
		` | spath input=event_json path=reason output=reason` +
		` | spath input=event_json path=message output=message` +
		` | spath input=event_json path=note output=note` +
		` | spath input=event_json path=metadata.namespace output=event_namespace` +
		` | spath input=event_json path=involvedObject.name output=core_name` +
		` | spath input=event_json path=involvedObject.kind output=core_kind` +
		` | spath input=event_json path=regarding.name output=regarding_name` +
		` | spath input=event_json path=regarding.kind output=regarding_kind` +
		` | spath input=event_json path=source.host output=node` +
		` | eval cluster=coalesce(deployment_cluster,cluster,"(missing)"),` +
		` kubernetes_namespace=coalesce(event_namespace,"(missing)"),` +
		` object_name=coalesce(core_name,regarding_name,"(missing)"),` +
		` object_kind=coalesce(core_kind,regarding_kind,"(missing)"),` +
		` message=coalesce(message,note,""), event_type=coalesce(event_type,"(missing)"),` +
		` reason=coalesce(reason,"(missing)"), node=coalesce(node,"(missing)")` +
		` | eval pod=if(object_kind="Pod",object_name,"(not a Pod)")` +
		` | fields _time host cluster kubernetes_namespace pod node object_kind object_name event_type reason message` +
		` | fields - _raw`
}

func buildCoverage(root string, profile contracts.SplunkProfileContract) (coverage, error) {
	report := coverage{
		Profile: profile.AppID, Status: "draft",
		Validation: "Generated parity is checked offline. Runtime evidence is documented per tested Splunk version; " +
			"SPL execution and Studio rendering require separate validation.",
		Semantics: "Mapped means a native search binding exists, not that its data source is present or qualified. " +
			"Audit counts use the selected range; metric windows are described per panel. " +
			"Focused native alert coverage is listed separately in alert-coverage.json.",
	}
	paths, err := filepath.Glob(filepath.Join(root, "contracts/dashboards/*.yaml"))
	if err != nil {
		return report, err
	}
	for _, path := range paths {
		canonical, err := contracts.LoadDashboardContract(path)
		if err != nil {
			return report, err
		}
		entry := dashboardCoverage{
			Source: "contracts/dashboards/" + filepath.Base(path), Status: "unavailable",
			IncludedPanels: []string{}, UnavailablePanels: []string{},
			RequiredSources: []string{},
		}
		for _, binding := range profile.Dashboards {
			if binding.Source == entry.Source {
				entry.Status = "partial"
				for _, panel := range binding.Panels {
					entry.IncludedPanels = append(entry.IncludedPanels, panel.SourcePanel)
					source := bindingSource(panel)
					if !slices.Contains(entry.RequiredSources, source) {
						entry.RequiredSources = append(entry.RequiredSources, source)
					}
				}
			}
		}
		for _, panel := range canonical.Panels {
			if !slices.Contains(entry.IncludedPanels, panel.ID) {
				entry.UnavailablePanels = append(entry.UnavailablePanels, panel.ID)
			}
		}
		if len(entry.UnavailablePanels) == 0 {
			entry.Status = "mapped"
		}
		report.Dashboards = append(report.Dashboards, entry)
	}
	return report, nil
}

func bindingSource(panel contracts.SplunkPanelBinding) string {
	for _, source := range []string{"audit", "operational", "completed", "platform"} {
		if strings.HasPrefix(panel.Search, "`openbao_"+source+"_events`") {
			return source + "-json"
		}
	}
	if panel.Scope != "" {
		return panel.Scope + "-metrics"
	}
	return "openbao-metrics"
}

func marshalJSON[T studioDashboard | coverage](document T) ([]byte, error) {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func dashboardInputNames(canonical contracts.DashboardContract, binding contracts.SplunkDashboardBinding) []string {
	signals := map[string]bool{}
	for _, panel := range canonical.Panels {
		for _, selected := range binding.Panels {
			if panel.ID == selected.SourcePanel {
				if strings.HasPrefix(selected.Search, auditMacro) {
					signals["audit"] = true
				} else {
					signals[panel.Signal] = true
				}
			}
		}
	}
	names := []string{}
	if signals["audit"] {
		names = append(names, "request_id", "request_path", "logical_namespace", "operation", "host")
	} else if signals["logs"] {
		names = append(names, "host")
	}
	if signals["metrics"] {
		names = append(names, "cluster", "instance", "scrape_profile", "kubernetes_namespace", "openbao_namespace")
	}
	for _, input := range binding.Inputs {
		if !slices.Contains(names, input.Name) {
			names = append(names, input.Name)
		}
	}
	return names
}

func panelPresentation(panel contracts.DashboardPanel) *visualOptions {
	unit := map[string]string{
		"ms": "ms", "s": "s", "ns": "ns", "bytes": "bytes", "ops": "operations/s", "reqps": "requests/s",
		"percent": "%", "percentunit": "ratio (0-1)", "cores": "CPU cores",
	}[panel.Unit]
	if panel.Type == "stat" && panel.Signal == "metrics" {
		return &visualOptions{
			Unit: unit, UnitPosition: "after", MajorValue: "> primary | seriesByName('value') | lastPoint()",
		}
	}
	if panel.Type == "timeseries" && unit != "" {
		return &visualOptions{YAxisTitleText: unit}
	}
	return nil
}
