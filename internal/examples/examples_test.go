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

func assertYAMLParses(t *testing.T, path string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open YAML: %v", err)
	}
	defer file.Close()

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

func isYAML(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".yaml" || extension == ".yml"
}
