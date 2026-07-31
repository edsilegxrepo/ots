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

func TestCustomizeDefaultMaxSecretSizeCalculation(t *testing.T) {
	// Verify defaultMaxSecretSize numeric bounds (65 MiB * 16 / 9 = 115.55 MiB = 121,168,782 bytes)
	expectedSize := int64(65 * 1024 * 1024 * 16 / 9)
	assert.Equal(t, int64(121168782), expectedSize)
	assert.Greater(t, defaultMaxSecretSize, int64(100*1024*1024), "defaultMaxSecretSize must be greater than 100 MiB to support 64 MiB attachments")

	c, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, expectedSize, c.MaxSecretSize)
}

func TestLoadTestDataReferenceCustomize(t *testing.T) {
	refPath := filepath.Join("..", "..", "testdata", "customize.yaml")
	if _, err := os.Stat(refPath); os.IsNotExist(err) {
		t.Skip("testdata/customize.yaml not found at path")
	}

	cust, err := Load(refPath)
	require.NoError(t, err, "testdata/customize.yaml must parse cleanly without errors")
	assert.Equal(t, "OTS - One Time Secrets", cust.AppTitle)
	assert.True(t, cust.DisablePoweredBy)
	assert.Len(t, cust.FooterLinks, 3)
	assert.Contains(t, cust.CustomBannerHTML, "Security Notice")
}
