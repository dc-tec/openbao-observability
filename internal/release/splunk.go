package release

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// SplunkAppOptions describes the installable app archive, without the trial runtime.
type SplunkAppOptions struct {
	RepositoryRoot  string
	Version         string
	OutputPath      string
	SourceDateEpoch int64
}

var appVersionPattern = regexp.MustCompile(`(?m)^version = [^\r\n]+$`)

// SplunkApp packages generated knowledge objects under their stable app ID.
// It stamps the release version in the archive without editing generated inputs.
func SplunkApp(opts SplunkAppOptions) error {
	if opts.Version == "" {
		opts.Version = "0.0.0-dev"
	}
	if opts.OutputPath == "" {
		opts.OutputPath = filepath.Join("dist", "release", "openbao_observability_"+opts.Version+".spl")
	}
	bundle, err := (BundleOptions{
		RepositoryRoot: opts.RepositoryRoot, Version: opts.Version,
		OutputPath: opts.OutputPath, SourceDateEpoch: opts.SourceDateEpoch,
	}).withDefaults()
	if err != nil {
		return err
	}
	appRoot := filepath.Join(bundle.RepositoryRoot, "generated", "splunk", "app", "openbao_observability")
	entries, err := collectBundleEntries(appRoot, []string{"default", "metadata"})
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(appRoot, "lookups")); err == nil {
		lookups, err := collectBundleEntries(appRoot, []string{"lookups"})
		if err != nil {
			return err
		}
		entries = append(entries, lookups...)
	} else if !os.IsNotExist(err) {
		return err
	}
	foundVersion := false
	for i := range entries {
		if entries[i].RelativePath != "default/app.conf" {
			continue
		}
		// #nosec G304 -- The CLI-selected repository root identifies generated release inputs.
		content, err := os.ReadFile(entries[i].AbsolutePath)
		if err != nil {
			return err
		}
		if len(appVersionPattern.FindAll(content, -1)) == 0 {
			return fmt.Errorf("generated Splunk app must declare a version")
		}
		entries[i].Content = appVersionPattern.ReplaceAllLiteral(content, []byte("version = "+bundle.Version))
		foundVersion = true
	}
	if !foundVersion {
		return fmt.Errorf("generated Splunk app lacks default/app.conf")
	}
	license, err := collectBundleEntries(bundle.RepositoryRoot, []string{"LICENSE"})
	if err != nil {
		return err
	}
	entries = append(entries, license...)
	return writeArchive(bundle.OutputPath, "openbao_observability", entries, bundle.SourceDateEpoch)
}
