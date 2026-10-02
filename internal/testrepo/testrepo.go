// Package testrepo builds throwaway repositories for tests.
package testrepo

import (
	"os"
	"path/filepath"
	"testing"
)

// New writes files (path -> content) under a fresh temporary directory.
func New(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	Write(t, root, files)
	return root
}

func Write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
