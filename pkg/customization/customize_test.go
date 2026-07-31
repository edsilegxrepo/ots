package customization

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomizeLoadAndToJSON(t *testing.T) {
	// Test Load with empty filename
	c1, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "OTS - One Time Secrets", c1.AppTitle)
	assert.Equal(t, int64(defaultMaxSecretSize), c1.MaxSecretSize)
	assert.True(t, c1.IsSearchIndexDisabled())

	// Test ToJSON
	jsonStr, err := c1.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonStr, `"appTitle":"OTS - One Time Secrets"`)

	// Test Load with custom file
	tmpDir := t.TempDir()
	custPath := filepath.Join(tmpDir, "customize.yaml")
	yamlContent := []byte(`
appTitle: "Custom OTS Title"
acceptedFileTypes: "@images, .pdf"
disableSearchIndex: false
`)
	require.NoError(t, os.WriteFile(custPath, yamlContent, 0o644))

	c2, err := Load(custPath)
	require.NoError(t, err)
	assert.Equal(t, "Custom OTS Title", c2.AppTitle)
	assert.False(t, c2.IsSearchIndexDisabled())
	assert.Contains(t, c2.ResolvedAcceptedExtensions, ".png")
	assert.Contains(t, c2.ResolvedAcceptedExtensions, ".pdf")
}
