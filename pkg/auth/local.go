package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"gopkg.in/yaml.v3"
)

const (
	argonMemory         = 64 * 1024 // 64 MB
	argonIterations     = 3
	argonParallelism    = 4
	argonSaltLen        = 16
	argonKeyLen         = 32
	argonHashPartsCount = 6
	maxDecodedHashLen   = 1024
	filePermUserOnly    = 0o600
)

// LocalAuthenticator manages local user accounts stored in users.yaml.
type LocalAuthenticator struct {
	usersFilePath string
	lastModTime   time.Time
	lock          sync.RWMutex
	directory     UserDirectory
}

var (
	// ErrInvalidCredentials is returned when username or password authentication fails.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrUserDisabled is returned when the user account is flagged as disabled.
	ErrUserDisabled = errors.New("user account is disabled")
	// ErrUserNotFound is returned when the specified user account does not exist.
	ErrUserNotFound = errors.New("user account not found")
)

// NewLocalAuthenticator creates a new local authenticator backed by users.yaml.
func NewLocalAuthenticator(usersFilePath string) (*LocalAuthenticator, error) {
	la := &LocalAuthenticator{
		usersFilePath: usersFilePath,
	}

	if usersFilePath != "" {
		if err := la.LoadUsers(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to load users file '%s': %w", usersFilePath, err)
		}
	}

	return la, nil
}

// AddUser adds or updates a user in the local directory.
func (la *LocalAuthenticator) AddUser(rec UserRecord) error {
	la.lock.Lock()
	defer la.lock.Unlock()

	if rec.Provider == "" {
		rec.Provider = "local"
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}

	found := false
	for i, u := range la.directory.Users {
		if strings.EqualFold(u.Username, rec.Username) {
			la.directory.Users[i] = rec
			found = true
			break
		}
	}

	if !found {
		la.directory.Users = append(la.directory.Users, rec)
	}

	return nil
}

// AuthenticateBasic parses and verifies HTTP Basic Auth credentials against local users.yaml directory.
func (la *LocalAuthenticator) AuthenticateBasic(r *http.Request) (*UserIdentity, error) {
	username, password, ok := r.BasicAuth()
	if !ok || username == "" {
		return nil, ErrInvalidCredentials
	}

	la.lock.Lock()
	if la.usersFilePath != "" {
		if info, err := os.Stat(la.usersFilePath); err == nil && info.ModTime().After(la.lastModTime) {
			_ = la.loadUsersLocked()
		}
	}
	defer la.lock.Unlock()

	for _, u := range la.directory.Users {
		if strings.EqualFold(u.Username, username) {
			if u.Disabled {
				return nil, ErrUserDisabled
			}

			if u.Hash != "" && !VerifyPassword(password, u.Hash) {
				return nil, ErrInvalidCredentials
			}

			return &UserIdentity{
				Username: u.Username,
				Email:    u.Email,
				Groups:   u.Groups,
				Provider: "local",
				AuthTime: time.Now(),
			}, nil
		}
	}

	return nil, ErrUserNotFound
}

// DeleteUser removes a user from the local directory atomically.
func (la *LocalAuthenticator) DeleteUser(username string) error {
	la.lock.Lock()

	newUsers := make([]UserRecord, 0, len(la.directory.Users))
	found := false
	for _, u := range la.directory.Users {
		if strings.EqualFold(u.Username, username) {
			found = true
			continue
		}
		newUsers = append(newUsers, u)
	}

	if !found {
		la.lock.Unlock()
		return ErrUserNotFound
	}

	la.directory.Users = newUsers
	la.lock.Unlock()

	return la.SaveUsers()
}

// ListUsers returns a copy of all user records in directory.
func (la *LocalAuthenticator) ListUsers() []UserRecord {
	la.lock.RLock()
	defer la.lock.RUnlock()

	result := make([]UserRecord, len(la.directory.Users))
	copy(result, la.directory.Users)
	return result
}

// LoadUsers reads and parses users.yaml from disk.
func (la *LocalAuthenticator) LoadUsers() error {
	la.lock.Lock()
	defer la.lock.Unlock()
	return la.loadUsersLocked()
}

// SaveUsers persists the current UserDirectory atomically to disk.
func (la *LocalAuthenticator) SaveUsers() error {
	la.lock.Lock()
	defer la.lock.Unlock()

	if la.usersFilePath == "" {
		return errors.New("no usersFilePath configured")
	}

	data, err := yaml.Marshal(&la.directory)
	if err != nil {
		return fmt.Errorf("failed to marshal user directory: %w", err)
	}

	tmpFile := la.usersFilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, filePermUserOnly); err != nil {
		return fmt.Errorf("failed to write temp user file: %w", err)
	}

	if err := os.Rename(tmpFile, la.usersFilePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to replace users file: %w", err)
	}

	if info, err := os.Stat(la.usersFilePath); err == nil {
		la.lastModTime = info.ModTime()
	}

	return nil
}

func (la *LocalAuthenticator) loadUsersLocked() error {
	if la.usersFilePath == "" {
		la.directory = UserDirectory{}
		return nil
	}

	info, err := os.Stat(la.usersFilePath)
	if err != nil {
		return fmt.Errorf("stat users file: %w", err)
	}

	data, err := os.ReadFile(la.usersFilePath)
	if err != nil {
		return fmt.Errorf("read users file: %w", err)
	}

	var dir UserDirectory
	if err := yaml.Unmarshal(data, &dir); err != nil {
		return fmt.Errorf("yaml parse error in users file: %w", err)
	}

	la.directory = dir
	la.lastModTime = info.ModTime()
	return nil
}

func decodeB64(s string) ([]byte, error) {
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	return b, nil
}

// HashPassword generates an OWASP-compliant Argon2id hash string for a plaintext password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonIterations, argonParallelism, b64Salt, b64Hash)

	return encoded, nil
}

// VerifyPassword checks if a plaintext password matches an encoded Argon2id hash.
func VerifyPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != argonHashPartsCount || parts[1] != "argon2id" {
		return false
	}

	var version int
	var memory uint32
	var iterations uint32
	var parallelism uint8

	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil || version != argon2.Version {
		return false
	}

	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false
	}

	salt, err := decodeB64(parts[4])
	if err != nil {
		return false
	}

	decodedHash, err := decodeB64(parts[5])
	if err != nil {
		return false
	}

	if len(decodedHash) > maxDecodedHashLen {
		return false
	}

	// #nosec G115 -- Length of decoded hash is explicitly bounded <= 1024 bytes above
	calculatedHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(decodedHash)))

	return subtle.ConstantTimeCompare(decodedHash, calculatedHash) == 1
}
