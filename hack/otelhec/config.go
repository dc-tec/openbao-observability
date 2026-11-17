package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	baselineImage = "otel/opentelemetry-collector-contrib:0.164.0@sha256:" +
		"340c9ca6a36617084c6661bdc4f0fe92b7cda5a805d61673ec8db375fddc522b"
	fixtureToken = "otel-hec-synthetic-fixture" // #nosec G101 -- Public dummy token accepted only by the synthetic sink.
	profilePath  = "examples/splunk/local/otel.yaml"
)

var imagePattern = regexp.MustCompile(`^otel/opentelemetry-collector-contrib:(\d+\.\d+\.\d+)@sha256:[a-f0-9]{64}$`)

func candidateImage(data []byte) (string, error) {
	var compose struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return "", err
	}
	value := compose.Services["splunk-otel"].Image
	const prefix = "${SPLUNK_OTEL_IMAGE:-"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "}") {
		return "", errors.New("splunk-otel must declare its default image")
	}
	image := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
	if !imagePattern.MatchString(image) {
		return "", errors.New("collector image must have a numeric version and immutable digest")
	}
	return image, nil
}

func mapping(values ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: values}
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func sequence(values ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: values}
}

func lookup(node *yaml.Node, path ...string) (*yaml.Node, error) {
	for _, key := range path {
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if node.Kind != yaml.MappingNode || next == nil {
			return nil, fmt.Errorf("missing profile mapping %q", key)
		}
		node = next
	}
	return node, nil
}

func put(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, scalar(key), value)
}

func selectComponents(root *yaml.Node, section string, names *yaml.Node) (*yaml.Node, error) {
	result := mapping()
	for _, name := range names.Content {
		component, err := lookup(root, section, name.Value)
		if err != nil {
			return nil, err
		}
		put(result, name.Value, component)
	}
	return result, nil
}

// fixtureConfig retains the real metrics pipeline and all relabel/transform rules.
// It replaces only fixture transport, scrape cadence and static deployment labels.
func fixtureConfig(source []byte, target, endpoint string) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(source, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 {
		return nil, errors.New("expected one collector profile document")
	}
	root := document.Content[0]
	pipeline, err := lookup(root, "service", "pipelines", "metrics")
	if err != nil {
		return nil, err
	}
	selected := mapping()
	for _, section := range []string{"receivers", "processors", "exporters"} {
		names, nameErr := lookup(pipeline, section)
		if nameErr != nil {
			return nil, nameErr
		}
		components, selectErr := selectComponents(root, section, names)
		if selectErr != nil {
			return nil, selectErr
		}
		put(selected, section, components)
	}
	if err := fixtureScrape(selected, target); err != nil {
		return nil, err
	}
	hec, err := lookup(selected, "exporters", "splunk_hec/metrics")
	if err != nil {
		return nil, err
	}
	put(hec, "endpoint", scalar(endpoint))
	put(hec, "token", scalar(fixtureToken))
	put(selected, "service", mapping(scalar("pipelines"), mapping(scalar("metrics"), pipeline)))
	return yaml.Marshal(selected)
}

func fixtureScrape(root *yaml.Node, target string) error {
	config, err := lookup(root, "receivers", "prometheus", "config")
	if err != nil {
		return err
	}
	jobs, err := lookup(config, "scrape_configs")
	if err != nil {
		return err
	}
	for _, job := range jobs.Content {
		name, nameErr := lookup(job, "job_name")
		if nameErr != nil || name.Value != "openbao" {
			continue
		}
		statics, staticErr := lookup(job, "static_configs")
		if staticErr != nil || len(statics.Content) == 0 {
			return errors.New("openbao scrape requires static deployment labels")
		}
		labels, labelsErr := lookup(statics.Content[0], "labels")
		if labelsErr != nil {
			return labelsErr
		}
		put(labels, "kubernetes_namespace", scalar("workloads"))
		put(job, "scrape_interval", scalar("250ms"))
		put(job, "scrape_timeout", scalar("250ms"))
		put(job, "static_configs", sequence(mapping(
			scalar("targets"), sequence(scalar(target)), scalar("labels"), labels,
		)))
		put(config, "scrape_configs", sequence(job))
		return nil
	}
	return errors.New("profile has no openbao scrape job")
}
