package metrics

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsCollectorAndHandler(t *testing.T) {
	collector := New()

	assert.NotNil(t, collector)

	// Exercise metric increment methods
	collector.CountSecretCreated()
	collector.CountSecretRead()
	collector.CountSecretCreateError("test_reason")
	collector.CountSecretReadError("not_found")
	collector.UpdateSecretsCount(42)

	// Test HTTP Handler
	handler := Handler()
	assert.Implements(t, (*http.Handler)(nil), handler)
}
