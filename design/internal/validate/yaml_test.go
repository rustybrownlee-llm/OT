package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

func TestLoadFile_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "device.yaml", `
schema_version: "0.1"
device:
  id: "test-device"
`)
	doc, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	if doc.SchemaVersion != "0.1" {
		t.Errorf("SchemaVersion = %q, want \"0.1\"", doc.SchemaVersion)
	}
	if doc.Device == nil || doc.Device.ID != "test-device" {
		t.Errorf("Device.ID unexpected: %v", doc.Device)
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "bad.yaml", ":\t:\ninot valid")
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name string
		doc  RawDocument
		want FileType
	}{
		{
			name: "device",
			doc:  RawDocument{Device: &DeviceDoc{ID: "x"}},
			want: FileTypeDevice,
		},
		{
			name: "network",
			doc:  RawDocument{Network: &NetworkDoc{ID: "x"}},
			want: FileTypeNetwork,
		},
		{
			name: "environment",
			doc:  RawDocument{Environment: &EnvironmentDoc{ID: "x"}},
			want: FileTypeEnvironment,
		},
		{
			name: "process",
			doc:  RawDocument{Process: &ProcessDoc{ID: "x"}},
			want: FileTypeProcess,
		},
		{
			name: "unknown",
			doc:  RawDocument{},
			want: FileTypeUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectFileType(&tc.doc)
			if got != tc.want {
				t.Errorf("DetectFileType() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindDesignRoot(t *testing.T) {
	root := t.TempDir()
	// Create design root structure.
	for _, sub := range []string{"devices", "networks", "environments"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Create a subdirectory inside environments.
	envDir := filepath.Join(root, "environments", "my-env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindDesignRoot(envDir)
	if err != nil {
		t.Fatalf("FindDesignRoot error: %v", err)
	}
	if got != root {
		t.Errorf("FindDesignRoot = %q, want %q", got, root)
	}
}

func TestFindDesignRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindDesignRoot(dir)
	if err == nil {
		t.Fatal("expected error when design root not found")
	}
}
