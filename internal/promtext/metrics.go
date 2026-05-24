package promtext

import (
	"fmt"
	"os"
	"slices"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

type Families map[string]*dto.MetricFamily

func LoadFamilies(path string) (Families, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Prometheus fixture %s: %w", path, err)
	}
	defer file.Close()

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(file)
	if err != nil {
		return nil, fmt.Errorf("parse Prometheus fixture %s: %w", path, err)
	}

	return Families(families), nil
}

func (f Families) HasMetric(name string) bool {
	_, ok := f[name]
	return ok
}

func (f Families) HasMetricWithLabel(name, labelName, labelValue string) bool {
	family, ok := f[name]
	if !ok {
		return false
	}

	for _, metric := range family.GetMetric() {
		if hasLabel(metric, labelName, labelValue) {
			return true
		}
	}

	return false
}

func (f Families) HasMetricWithLabelName(name, labelName string) bool {
	family, ok := f[name]
	if !ok {
		return false
	}

	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == labelName {
				return true
			}
		}
	}

	return false
}

func (f Families) Names() []string {
	names := make([]string, 0, len(f))
	for name := range f {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func hasLabel(metric *dto.Metric, labelName, labelValue string) bool {
	for _, label := range metric.GetLabel() {
		if label.GetName() == labelName && label.GetValue() == labelValue {
			return true
		}
	}
	return false
}
