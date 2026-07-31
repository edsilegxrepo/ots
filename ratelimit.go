// Package main - Sliding Window IP Rate Limiter & IP Extraction Utilities
//
// Objectives:
// - Protects public API creation endpoints (/api/create) against automated abuse, brute force attacks, and spam.
// - Implements a thread-safe sliding window algorithm per client IP address.
// - Provides a background periodic pruner to clean up stale IP entries and prevent memory growth.
//
// Core Components:
// - ipRateLimiter: Mutex-protected struct tracking request timestamps per client IP.
// - Allow: Evaluates whether an IP address is within configured rate limits for the current time window.
// - cleanup: Background routine running every 5 minutes to purge expired IP entries.
// - getClientIP: Extracts client IP address from proxy headers (X-Forwarded-For, X-Real-IP) or RemoteAddr.
package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ipRateLimiter manages in-memory sliding window request counts for client IP addresses.
type ipRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// newIPRateLimiter constructs an ipRateLimiter and launches a background cleanup goroutine.
func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	limiter := &ipRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	go func() {
		for t := time.NewTicker(5 * time.Minute); ; <-t.C {
			limiter.cleanup()
		}
	}()

	return limiter
}

// Allow checks whether the specified IP has exceeded the allowed request limit in the current sliding window.
func (l *ipRateLimiter) Allow(ip string) bool {
	if l.limit <= 0 {
		return true // Rate limiting disabled
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	var valid []time.Time
	for _, t := range l.requests[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.limit {
		l.requests[ip] = valid
		return false
	}

	valid = append(valid, now)
	l.requests[ip] = valid
	return true
}

// cleanup removes expired timestamp entries from the in-memory rate limiter map.
func (l *ipRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.window)
	for ip, timestamps := range l.requests {
		var valid []time.Time
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.requests, ip)
		} else {
			l.requests[ip] = valid
		}
	}
}

// getClientIP extracts the client's real IP address from proxy headers (X-Forwarded-For, X-Real-IP) or TCP RemoteAddr.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
