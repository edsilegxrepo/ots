package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIPRateLimiter(t *testing.T) {
	limiter := newIPRateLimiter(3, 1*time.Minute)

	ip := "192.168.1.100"

	assert.True(t, limiter.Allow(ip))
	assert.True(t, limiter.Allow(ip))
	assert.True(t, limiter.Allow(ip))
	assert.False(t, limiter.Allow(ip)) // 4th request rejected
}

func TestGetClientIP(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")

	ip := getClientIP(req)
	assert.Equal(t, "203.0.113.195", ip)
}
