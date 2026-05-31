package api

import (
	"path/filepath"
	"testing"
)

func TestImportJobTempDirIsAbsolute(t *testing.T) {
	manager := NewImportManager(nil, ImportManagerConfig{TempDir: "data/imports"})

	got, err := manager.jobTempDir("import-1")
	if err != nil {
		t.Fatalf("jobTempDir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("jobTempDir = %q, want absolute path", got)
	}
	if filepath.Base(got) != "import-1" {
		t.Fatalf("jobTempDir = %q, want import id as last path element", got)
	}
}
