// Package customization contains the structure for the customization
// file to configure the OTS web- and command-line interface.
//
// Objectives:
// - Manages loading, validation, and JSON serialization of custom operator configuration files (customize.yaml).
// - Sets sensible production defaults for secret sizes, rate limits, search engine privacy (robots.txt), and UI settings.
// - Resolves group alias tokens (@images, @office, @archives) into normalized extension lists for pre-flight attachment checks.
//
// Core Components:
// - Customize: Central configuration struct containing UI settings, file attachment rules, rate limits, and privacy toggles.
// - Load: Reads and unmarshals YAML configuration files from the filesystem.
// - ApplyFixes: Applies default values and resolves group aliases via ExpandAcceptedFileTypes.
// - IsSearchIndexDisabled: Evaluates privacy settings to control search engine indexing (Issue #221).
// - ToJSON: Template helper serializing customization settings for embedded web assets and API endpoints.
package customization

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Frontend has a max attachment size of 64MiB as the base64 encoding
// will break afterwards. Therefore we use a maximum secret size of
// 65MiB and increase it by double base64 encoding:
//
// 65 MiB * 16/9 (twice 4/3 base64 size increase)
const defaultMaxSecretSize = 65 * 1024 * 1024 * 16 / 9 // = 115.55MiB

type (
	// Customize holds the structure of the customization file
	Customize struct {
		AppIcon              string `json:"appIcon,omitempty" yaml:"appIcon"`
		AppIconDark          string `json:"appIconDark,omitempty" yaml:"appIconDark"`
		AppTitle             string `json:"appTitle,omitempty" yaml:"appTitle"`
		CustomBannerHTML     string `json:"customBannerHTML,omitempty" yaml:"customBannerHTML"`
		DisableAppTitle      bool   `json:"disableAppTitle,omitempty" yaml:"disableAppTitle"`
		DisableDefaultExpiry bool   `json:"disableDefaultExpiry,omitempty" yaml:"disableDefaultExpiry"`
		DisablePoweredBy     bool   `json:"disablePoweredBy,omitempty" yaml:"disablePoweredBy"`
		DisableQRSupport     bool   `json:"disableQRSupport,omitempty" yaml:"disableQRSupport"`
		DisableSearchIndex   *bool  `json:"disableSearchIndex,omitempty" yaml:"disableSearchIndex"`
		DisableThemeSwitcher bool   `json:"disableThemeSwitcher,omitempty" yaml:"disableThemeSwitcher"`

		DisableExpiryOverride bool    `json:"disableExpiryOverride,omitempty" yaml:"disableExpiryOverride"`
		ExpiryChoices         []int64 `json:"expiryChoices,omitempty" yaml:"expiryChoices"`

		AcceptedFileTypes          string   `json:"acceptedFileTypes" yaml:"acceptedFileTypes"`
		ResolvedAcceptedExtensions []string `json:"resolvedAcceptedExtensions,omitempty" yaml:"-"`
		DisableFileAttachment      bool     `json:"disableFileAttachment" yaml:"disableFileAttachment"`
		MaxAttachmentSizeTotal     int64    `json:"maxAttachmentSizeTotal" yaml:"maxAttachmentSizeTotal"`

		FileGroupsPath        string        `json:"-" yaml:"fileGroupsPath"`
		MaxSecretSize         int64         `json:"-" yaml:"maxSecretSize"`
		MetricsAllowedSubnets []string      `json:"-" yaml:"metricsAllowedSubnets"`
		OverlayFSPath         string        `json:"-" yaml:"overlayFSPath"`
		RateLimitCreate       int           `json:"-" yaml:"rateLimitCreate"`
		TrustedProxies        []string      `json:"-" yaml:"trustedProxies"`
		ResolvedTrustedCIDRs  []*net.IPNet  `json:"-" yaml:"-"`
		ResolvedTrustedIPs    []net.IP      `json:"-" yaml:"-"`
		UseFormalLanguage     bool          `json:"-" yaml:"useFormalLanguage"`

		FooterLinks []FooterLink `json:"footerLinks,omitempty" yaml:"footerLinks"`
	}

	// FooterLink holds name/url combinations to add as a link in the
	// footer to i.e. add imprint or privacy policy
	FooterLink struct {
		Name string `json:"name" yaml:"name"`
		URL  string `json:"url" yaml:"url"`
	}
)

// Load retrieves the Customization file from filesystem
func Load(filename string) (cust Customize, err error) {
	if filename == "" {
		// None given, take a shortcut
		cust.ApplyFixes()
		return cust, nil
	}

	cf, err := os.Open(filename) //#nosec:G304 // Loading a custom file is the intention here
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logrus.Warn("customize file given but not found")
			return cust, nil
		}
		return cust, fmt.Errorf("opening customize file: %w", err)
	}
	defer func() {
		if err := cf.Close(); err != nil {
			logrus.WithError(err).Error("closing customize file (leaked fd)")
		}
	}()

	if err = yaml.NewDecoder(cf).Decode(&cust); err != nil {
		return cust, fmt.Errorf("decoding customize file: %w", err)
	}

	cust.ApplyFixes()

	return cust, nil
}

// ToJSON is a templating helper which returns the customization
// serialized as JSON in a string
func (c Customize) ToJSON() (string, error) {
	j, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshalling JSON: %w", err)
	}

	return string(j), nil
}

// IsSearchIndexDisabled returns true if search index is disabled (defaults to true for privacy).
func (c Customize) IsSearchIndexDisabled() bool {
	if c.DisableSearchIndex == nil {
		return true
	}
	return *c.DisableSearchIndex
}

func (c *Customize) ApplyFixes() {
	if len(c.AppTitle) == 0 {
		c.AppTitle = "OTS - One Time Secrets"
	}

	if c.MaxSecretSize == 0 {
		c.MaxSecretSize = defaultMaxSecretSize
	}

	if c.RateLimitCreate == 0 {
		c.RateLimitCreate = 30
	}

	if c.DisableSearchIndex == nil {
		defaultDisable := true
		c.DisableSearchIndex = &defaultDisable
	}

	var customGroups map[string][]string
	if c.FileGroupsPath != "" {
		var err error
		if customGroups, err = LoadCustomFileGroups(c.FileGroupsPath); err != nil {
			logrus.WithError(err).Warn("failed to load custom file groups file")
		}
	}
	c.ResolvedAcceptedExtensions = ExpandAcceptedFileTypes(c.AcceptedFileTypes, customGroups)

	c.ResolvedTrustedCIDRs = nil
	c.ResolvedTrustedIPs = nil
	for _, entry := range c.TrustedProxies {
		_, cidr, err := net.ParseCIDR(entry)
		if err == nil {
			c.ResolvedTrustedCIDRs = append(c.ResolvedTrustedCIDRs, cidr)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			c.ResolvedTrustedIPs = append(c.ResolvedTrustedIPs, ip)
			continue
		}
		logrus.WithField("proxy", entry).Warn("invalid trusted proxy entry in customization file")
	}
}
