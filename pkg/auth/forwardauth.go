package auth

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	ErrUntrustedProxy = errors.New("untrusted proxy IP: forwardauth headers rejected")
	ErrMissingUser    = errors.New("missing authenticated user header")
)

// ForwardAuthAuthenticator evaluates HTTP headers injected by reverse proxies (Authelia, Authentik, OAuth2-Proxy, Pomerium, Okta).
type ForwardAuthAuthenticator struct {
	config ForwardAuthConfig
	nets   []*net.IPNet
	ips    []net.IP
}

// NewForwardAuthAuthenticator initializes a new ForwardAuth header authenticator.
func NewForwardAuthAuthenticator(cfg ForwardAuthConfig) (*ForwardAuthAuthenticator, error) {
	if len(cfg.TrustedProxies) == 0 {
		return nil, fmt.Errorf("forwardauth requires non-empty trustedProxies list to prevent header spoofing")
	}

	fa := &ForwardAuthAuthenticator{
		config: cfg,
		nets:   make([]*net.IPNet, 0),
		ips:    make([]net.IP, 0),
	}

	if cfg.UserHeader == "" {
		fa.config.UserHeader = "Remote-User"
	}
	if cfg.EmailHeader == "" {
		fa.config.EmailHeader = "Remote-Email"
	}
	if cfg.GroupsHeader == "" {
		fa.config.GroupsHeader = "Remote-Groups"
	}
	if cfg.HeaderDelimiter == "" {
		fa.config.HeaderDelimiter = ","
	}

	for _, proxyStr := range cfg.TrustedProxies {
		proxyStr = strings.TrimSpace(proxyStr)
		if proxyStr == "" {
			continue
		}
		if strings.Contains(proxyStr, "/") {
			_, ipNet, err := net.ParseCIDR(proxyStr)
			if err != nil {
				return nil, fmt.Errorf("invalid trustedProxy CIDR '%s': %w", proxyStr, err)
			}
			fa.nets = append(fa.nets, ipNet)
		} else {
			ip := net.ParseIP(proxyStr)
			if ip == nil {
				return nil, fmt.Errorf("invalid trustedProxy IP '%s'", proxyStr)
			}
			fa.ips = append(fa.ips, ip)
		}
	}

	return fa, nil
}

// IsTrustedProxy verifies whether remoteAddr belongs to configured trustedProxies.
func (fa *ForwardAuthAuthenticator) IsTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}

	for _, trustedIP := range fa.ips {
		if trustedIP.Equal(ip) {
			return true
		}
	}

	for _, trustedNet := range fa.nets {
		if trustedNet.Contains(ip) {
			return true
		}
	}

	return false
}

// Authenticate extracts identity from incoming HTTP request headers if remoteAddr is a trusted proxy.
func (fa *ForwardAuthAuthenticator) Authenticate(r *http.Request) (*UserIdentity, error) {
	if !fa.IsTrustedProxy(r.RemoteAddr) {
		return nil, ErrUntrustedProxy
	}

	username := strings.TrimSpace(r.Header.Get(fa.config.UserHeader))
	if username == "" {
		return nil, ErrMissingUser
	}

	email := strings.TrimSpace(r.Header.Get(fa.config.EmailHeader))
	rawGroups := strings.TrimSpace(r.Header.Get(fa.config.GroupsHeader))

	groups := make([]string, 0)
	if rawGroups != "" {
		parts := strings.Split(rawGroups, fa.config.HeaderDelimiter)
		for _, p := range parts {
			clean := strings.TrimSpace(p)
			if clean != "" {
				groups = append(groups, clean)
			}
		}
	}

	return &UserIdentity{
		Username: username,
		Email:    email,
		Groups:   groups,
		Provider: "forwardauth",
		AuthTime: time.Now(),
	}, nil
}
