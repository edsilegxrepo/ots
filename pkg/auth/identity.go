package auth

import (
	"time"

	"gopkg.in/yaml.v3"
)

// UserIdentity represents a normalized authenticated caller in the OTS engine.
type UserIdentity struct {
	Username string    `json:"username"`
	Email    string    `json:"email,omitempty"`
	Groups   []string  `json:"groups"`
	Provider string    `json:"provider"` // "forwardauth", "local", "htpasswd"
	AuthTime time.Time `json:"authTime"`
}

// UserRecord represents an account stored in /etc/ots/users.yaml.
type UserRecord struct {
	Username  string    `yaml:"username" json:"username"`
	Provider  string    `yaml:"provider" json:"provider"` // Default: "local"
	Hash      string    `yaml:"hash,omitempty" json:"hash,omitempty"`
	Email     string    `yaml:"email,omitempty" json:"email,omitempty"`
	Groups    []string  `yaml:"groups" json:"groups"`
	Disabled  bool      `yaml:"disabled" json:"disabled"`
	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
}

// UserDirectory represents the structure of /etc/ots/users.yaml.
type UserDirectory struct {
	Users []UserRecord `yaml:"users" json:"users"`
}

// FeaturePolicy defines capability-specific group permissions in iam.yaml.
type FeaturePolicy struct {
	AllowedGroups []string `yaml:"allowedGroups" json:"allowedGroups"`
	MaxSizeBytes  int64    `yaml:"maxSizeBytes,omitempty" json:"maxSizeBytes,omitempty"`
}

// IAMPolicy defines global and feature authorization rules in iam.yaml.
type IAMPolicy struct {
	DefaultPolicy   string                   `yaml:"defaultPolicy" json:"defaultPolicy"` // "deny" or "allow"
	AllowedGroups   []string                 `yaml:"allowedGroups" json:"allowedGroups"`
	FeaturePolicies map[string]FeaturePolicy `yaml:"featurePolicies,omitempty" json:"featurePolicies,omitempty"`
}

// ForwardAuthConfig holds settings for the ForwardAuth HTTP header connector.
type ForwardAuthConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	UserHeader      string   `yaml:"userHeader" json:"userHeader"`           // Default: "Remote-User"
	EmailHeader     string   `yaml:"emailHeader" json:"emailHeader"`         // Default: "Remote-Email"
	GroupsHeader    string   `yaml:"groupsHeader" json:"groupsHeader"`       // Default: "Remote-Groups"
	HeaderDelimiter string   `yaml:"headerDelimiter" json:"headerDelimiter"` // Default: ","
	TrustedProxies  []string `yaml:"trustedProxies" json:"trustedProxies"`
}

// LocalAuthConfig holds settings for local user authentication.
type LocalAuthConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// HTPasswdConfig holds settings for Apache htpasswd file authentication.
type HTPasswdConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	File    string `yaml:"file" json:"file"`
}

// IAMConnectors holds all configured authentication connectors.
type IAMConnectors struct {
	Local       LocalAuthConfig   `yaml:"local" json:"local"`
	HTPasswd    HTPasswdConfig    `yaml:"htpasswd" json:"htpasswd"`
	ForwardAuth ForwardAuthConfig `yaml:"forwardauth" json:"forwardauth"`
}

// IAMConfig represents the root configuration structure for iam.yaml.
type IAMConfig struct {
	Enabled            bool          `yaml:"enabled" json:"enabled"`
	ProtectedEndpoints []string      `yaml:"protectedEndpoints" json:"protectedEndpoints"`
	UsersFilePath      string        `yaml:"usersFilePath" json:"usersFilePath"`
	Connector          string        `yaml:"connector" json:"connector"` // "forwardauth", "local", "htpasswd"
	Policy             IAMPolicy     `yaml:"policy" json:"policy"`
	Connectors         IAMConnectors `yaml:"connectors" json:"connectors"`
}

// IAMConfigFileWrapper handles unmarshaling files wrapped under the top-level 'iam:' YAML key.
type IAMConfigFileWrapper struct {
	IAM IAMConfig `yaml:"iam" json:"iam"`
}

// LoadIAMConfig reads and unmarshals iam.yaml supporting both top-level 'iam:' and direct root schemas.
func LoadIAMConfig(data []byte) (IAMConfig, error) {
	var wrapper IAMConfigFileWrapper
	if err := yaml.Unmarshal(data, &wrapper); err == nil && (wrapper.IAM.Enabled || wrapper.IAM.Connector != "") {
		return wrapper.IAM, nil
	}

	var direct IAMConfig
	if err := yaml.Unmarshal(data, &direct); err != nil {
		return IAMConfig{}, err
	}

	return direct, nil
}
