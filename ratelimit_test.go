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
	cust.TrustedProxies = nil
	cust.ResolvedTrustedCIDRs = nil
	cust.ResolvedTrustedIPs = nil

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")

	ip := getClientIP(req)
	assert.Equal(t, "203.0.113.195", ip)
}

func TestGetClientIPTrustedProxies(t *testing.T) {
	cust.TrustedProxies = []string{"10.0.0.0/8"}
	cust.ApplyFixes()

	// Trusted proxy source (10.0.0.5) -> X-Forwarded-For honored
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.5:12345"
	req1.Header.Set("X-Forwarded-For", "203.0.113.195")
	assert.Equal(t, "203.0.113.195", getClientIP(req1))

	// Untrusted source (198.51.100.2) -> X-Forwarded-For ignored (anti-spoofing)
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req2.RemoteAddr = "198.51.100.2:12345"
	req2.Header.Set("X-Forwarded-For", "203.0.113.195")
	assert.Equal(t, "198.51.100.2", getClientIP(req2))

	// Reset cust.TrustedProxies
	cust.TrustedProxies = nil
	cust.ApplyFixes()
}

func TestIPRateLimiterSharding(t *testing.T) {
	limiter := newIPRateLimiter(2, 1*time.Minute)

	ips := []string{"192.168.1.1", "10.0.0.2", "172.16.0.3", "1.1.1.1"}
	for _, ip := range ips {
		assert.True(t, limiter.Allow(ip))
		assert.True(t, limiter.Allow(ip))
		assert.False(t, limiter.Allow(ip)) // 3rd request per IP rejected
	}
}
