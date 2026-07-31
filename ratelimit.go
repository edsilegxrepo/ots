// Package main - Sliding Window Sharded IP Rate Limiter & Trusted Proxy IP Extraction
//
// Objectives:
// - Protects public API creation endpoints (/api/create) against automated abuse, brute force attacks, and spam.
// - Implements a thread-safe sliding window algorithm per client IP address using sharded mutex buckets to minimize lock contention.
// - Provides a background periodic pruner to clean up stale IP entries and prevent memory growth.
// - Safely extracts client IP address only when requests originate from trusted reverse proxies.
package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const numRateLimiterShards = 32

type ipRateLimiterShard struct {
	mu       sync.Mutex
	requests map[string][]time.Time
}

// ipRateLimiter manages sharded in-memory sliding window request counts for client IP addresses.
type ipRateLimiter struct {
	shards []*ipRateLimiterShard
	limit  int
	window time.Duration
}

// newIPRateLimiter constructs an ipRateLimiter and launches a background cleanup goroutine.
func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	limiter := &ipRateLimiter{
		shards: make([]*ipRateLimiterShard, numRateLimiterShards),
		limit:  limit,
		window: window,
	}

	for i := 0; i < numRateLimiterShards; i++ {
		limiter.shards[i] = &ipRateLimiterShard{
			requests: make(map[string][]time.Time),
		}
	}

	go func() {
		for t := time.NewTicker(5 * time.Minute); ; <-t.C {
			limiter.cleanup()
		}
	}()

	return limiter
}

func (l *ipRateLimiter) getShard(ip string) *ipRateLimiterShard {
	var h uint32 = 2166136261
	for i := 0; i < len(ip); i++ {
		h ^= uint32(ip[i])
		h *= 16777619
	}
	return l.shards[h%numRateLimiterShards]
}

// Allow checks whether the specified IP has exceeded the allowed request limit in the current sliding window.
func (l *ipRateLimiter) Allow(ip string) bool {
	if l.limit <= 0 {
		return true // Rate limiting disabled
	}

	shard := l.getShard(ip)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	existing := shard.requests[ip]
	valid := make([]time.Time, 0, len(existing)+1)
	for _, t := range existing {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.limit {
		shard.requests[ip] = valid
		return false
	}

	valid = append(valid, now)
	shard.requests[ip] = valid
	return true
}

// cleanup removes expired timestamp entries from all sharded rate limiter maps.
func (l *ipRateLimiter) cleanup() {
	cutoff := time.Now().Add(-l.window)

	for _, shard := range l.shards {
		shard.mu.Lock()
		for ip, timestamps := range shard.requests {
			var valid []time.Time
			for _, t := range timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(shard.requests, ip)
			} else {
				shard.requests[ip] = valid
			}
		}
		shard.mu.Unlock()
	}
}

// getClientIP extracts the client's real IP address from proxy headers if RemoteAddr originates from trusted proxies.
func getClientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	remoteIP := net.ParseIP(remoteHost)

	// Default isTrusted to true if trustedProxies is not configured (backward compatibility)
	isTrusted := len(cust.TrustedProxies) == 0
	if len(cust.TrustedProxies) > 0 && remoteIP != nil {
		isTrusted = false
		for _, cidr := range cust.ResolvedTrustedCIDRs {
			if cidr.Contains(remoteIP) {
				isTrusted = true
				break
			}
		}
		if !isTrusted {
			for _, singleIP := range cust.ResolvedTrustedIPs {
				if singleIP.Equal(remoteIP) {
					isTrusted = true
					break
				}
			}
		}
	}

	if isTrusted {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	return remoteHost
}
