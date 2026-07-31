package customization

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpandAcceptedFileTypes(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		customGroups map[string][]string
		expected     []string
	}{
		{
			name:     "Empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "Individual extensions mixed formats",
			input:    ".png, JPG, * .pdf, 7z",
			expected: []string{".png", ".jpg", ".pdf", ".7z"},
		},
		{
			name:     "Group alias @images",
			input:    "@images",
			expected: []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico", ".tiff", ".heic"},
		},
		{
			name:  "Group alias @packages and @binaries",
			input: "@packages, @binaries",
			expected: []string{
				".deb", ".rpm", ".apk", ".msi", ".pkg", ".appimage", ".dmg", ".flatpakref", ".snap", ".ipa",
				".exe", ".bin", ".dll", ".so", ".dylib", ".elf", ".dat",
			},
		},
		{
			name:  "Group alias @office and @archives",
			input: "@office, @archives",
			expected: []string{
				".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".txt", ".rtf", ".csv",
				".zip", ".7z", ".rar", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".iso", ".cab", ".zst",
			},
		},
		{
			name:  "Custom group override",
			input: "@custom, .txt",
			customGroups: map[string][]string{
				"@custom": {".foo", ".bar"},
			},
			expected: []string{".foo", ".bar", ".txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandAcceptedFileTypes(tt.input, tt.customGroups)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExpandAcceptedFileTypes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsFilenameAllowed(t *testing.T) {
	allowed := []string{".png", ".jpg", ".7z", ".deb", ".rpm", ".tar.gz"}

	tests := []struct {
		filename string
		expected bool
	}{
		{"image.PNG", true},
		{"photo.jpg", true},
		{"archive.7z", true},
		{"package.rpm", true},
		{"bundle.tar.gz", true},
		{"script.sh", false},
		{"malware.exe", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := IsFilenameAllowed(tt.filename, allowed)
			if got != tt.expected {
				t.Errorf("IsFilenameAllowed(%q) = %v, want %v", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestLoadCustomFileGroups(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "custom_groups.json")

	content := []byte(`{
		"cad": ["dwg", ".dxf"],
		"@3d": [".stl", "obj"]
	}`)

	if err := os.WriteFile(jsonPath, content, 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	groups, err := LoadCustomFileGroups(jsonPath)
	if err != nil {
		t.Fatalf("LoadCustomFileGroups returned error: %v", err)
	}

	expected := map[string][]string{
		"@cad": {".dwg", ".dxf"},
		"@3d":  {".stl", ".obj"},
	}

	if !reflect.DeepEqual(groups, expected) {
		t.Errorf("LoadCustomFileGroups() = %v, want %v", groups, expected)
	}
}
