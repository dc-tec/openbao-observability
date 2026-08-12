package contracts

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"
)

var metricSourcePrefixPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type contractDocument interface {
	MetricContract | AlertContract | StreamContract | DashboardContract
}

func decodeContractYAML[T contractDocument](content []byte, target *T) error {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("contract must contain exactly one YAML document")
	}

	return nil
}

func validateContractVersion(path, version string) error {
	if version == "" {
		return fmt.Errorf("contract %s is missing version", path)
	}
	if version != contractSchemaVersion {
		return fmt.Errorf("contract %s has unsupported version %q; want %q", path, version, contractSchemaVersion)
	}
	return nil
}

func validateMetricSourcePrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("metric source prefix is empty")
	}
	if !metricSourcePrefixPattern.MatchString(prefix) {
		return fmt.Errorf("metric source prefix %q is not a valid Prometheus metric prefix", prefix)
	}
	return nil
}
