package examples

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExampleYAMLParses(t *testing.T) {
	root := filepath.Join("..", "..", "examples")

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isYAML(path) {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			assertYAMLParses(t, path)
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk examples: %v", err)
	}
}

func TestKubernetesExamplesAvoidUnsafeDefaults(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "kubernetes")

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isYAML(path) {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			assertKubernetesExampleAvoidsUnsafeDefaults(t, path)
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk Kubernetes examples: %v", err)
	}
}

func TestMetricsScrapeExamplesNormalizeSignalIdentity(t *testing.T) {
	tests := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{
			path: filepath.Join("..", "..", "examples", "docker-compose", "prometheus", "prometheus.yml"),
			required: []string{
				"cluster: compose",
				"scrape_profile: all_nodes",
				"target_label: pod",
				"target_label: openbao_namespace",
				`regex: "namespace|exported_cluster"`,
			},
		},
		{
			path: filepath.Join("..", "..", "examples", "kubernetes", "secure-metrics-scrape.yaml"),
			required: []string{
				"honorLabels: false",
				"targetLabel: cluster",
				"targetLabel: kubernetes_namespace",
				"targetLabel: pod",
				"targetLabel: scrape_profile",
				"targetLabel: openbao_namespace",
			},
			forbidden: []string{"honorLabels: true"},
		},
		{
			path: filepath.Join("..", "..", "examples", "kubernetes", "all-node-metrics-scrape.yaml"),
			required: []string{
				"honorLabels: false",
				"targetLabel: cluster",
				"targetLabel: kubernetes_namespace",
				"targetLabel: pod",
				"targetLabel: scrape_profile",
				"targetLabel: openbao_namespace",
			},
			forbidden: []string{"honorLabels: true"},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			content, err := os.ReadFile(test.path)
			if err != nil {
				t.Fatalf("read example: %v", err)
			}
			text := string(content)
			for _, fragment := range test.required {
				if !strings.Contains(text, fragment) {
					t.Errorf("example does not contain %q", fragment)
				}
			}
			for _, fragment := range test.forbidden {
				if strings.Contains(text, fragment) {
					t.Errorf("example contains forbidden value %q", fragment)
				}
			}
		})
	}
}

func TestComposeAlloyUsesContractLogLabels(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "docker-compose", "alloy", "config.alloy")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Alloy configuration: %v", err)
	}
	text := string(content)

	for _, node := range []string{"openbao-node0", "openbao-node1", "openbao-node2"} {
		for _, label := range []string{"instance", "pod"} {
			fragment := fmt.Sprintf(`%q`, label)
			value := fmt.Sprintf(`= %q`, node)
			if !strings.Contains(text, fragment) || !strings.Contains(text, value) {
				t.Errorf("Alloy configuration does not contain %s for %s", label, node)
			}
		}
	}
	if strings.Contains(text, `"job"`) {
		t.Error("Alloy configuration emits the uncontracted job label")
	}
}

func assertYAMLParses(t *testing.T, path string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open YAML: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close YAML: %v", err)
		}
	}()

	decoder := yaml.NewDecoder(file)
	documents := 0
	for {
		var document any
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode YAML document %d: %v", documents+1, err)
		}
		if document != nil {
			documents++
		}
	}

	if documents == 0 {
		t.Fatal("YAML file does not contain any documents")
	}
}

func assertKubernetesExampleAvoidsUnsafeDefaults(t *testing.T, path string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open YAML: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close YAML: %v", err)
		}
	}()

	decoder := yaml.NewDecoder(file)
	for {
		var document any
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode YAML: %v", err)
		}

		assertNoUnsafeKubernetesExampleValue(t, path, document)
	}
}

func assertNoUnsafeKubernetesExampleValue(t *testing.T, path string, value any) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "kind" && child == "Secret" {
				t.Fatalf("%s embeds a Secret; create tokens and certificates outside reusable examples", path)
			}
			if key == "insecureSkipVerify" && child == true {
				t.Fatalf("%s sets insecureSkipVerify: true", path)
			}
			assertNoUnsafeKubernetesExampleValue(t, path, child)
		}
	case []any:
		for _, child := range typed {
			assertNoUnsafeKubernetesExampleValue(t, path, child)
		}
	}
}

func isYAML(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".yaml" || extension == ".yml"
}
