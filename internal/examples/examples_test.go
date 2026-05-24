package examples

import (
	"errors"
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
