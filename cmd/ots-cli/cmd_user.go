package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/edsilegxrepo/ots/pkg/auth"
	"github.com/edsilegxrepo/ots/pkg/client"
)

var (
	userCmd = &cobra.Command{
		Use:   "user",
		Short: "Manage local user accounts and IAM role mappings",
	}

	userAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Provision a new user account in users.yaml",
		RunE:  runUserAdd,
	}

	userListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all provisioned user accounts",
		RunE:  runUserList,
	}

	userDisableCmd = &cobra.Command{
		Use:   "disable",
		Short: "Disable a user account",
		RunE:  runUserDisable,
	}

	userDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a user account from users.yaml",
		RunE:  runUserDelete,
	}
)

func init() {
	rootCmd.AddCommand(userCmd)

	userCmd.AddCommand(userAddCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userDisableCmd)
	userCmd.AddCommand(userDeleteCmd)

	// User Add Flags
	userAddCmd.Flags().String("username", "", "Username for the new account (required)")
	userAddCmd.Flags().String("email", "", "Email address for the user")
	userAddCmd.Flags().String("groups", "", "Comma-separated list of RBAC groups (e.g. 'DevOps,OTS-Creators')")
	userAddCmd.Flags().String("provider", "local", "Authentication provider ('local' or 'forwardauth')")
	userAddCmd.Flags().String("users-file", "testfiles/users.yaml", "Path to users.yaml file")
	userAddCmd.Flags().Bool("password-stdin", false, "Read explicit password from STDIN")
	userAddCmd.Flags().Bool("create-ots-link", false, "Generate single-use OTS secret link containing onboarding credentials")
	userAddCmd.Flags().String("ots-url", "http://127.0.0.1:3000", "OTS server instance URL for creating secret link")

	_ = userAddCmd.MarkFlagRequired("username")

	// User List Flags
	userListCmd.Flags().String("users-file", "testfiles/users.yaml", "Path to users.yaml file")

	// User Disable Flags
	userDisableCmd.Flags().String("username", "", "Username to disable (required)")
	userDisableCmd.Flags().String("users-file", "testfiles/users.yaml", "Path to users.yaml file")
	_ = userDisableCmd.MarkFlagRequired("username")

	// User Delete Flags
	userDeleteCmd.Flags().String("username", "", "Username to delete (required)")
	userDeleteCmd.Flags().String("users-file", "testfiles/users.yaml", "Path to users.yaml file")
	_ = userDeleteCmd.MarkFlagRequired("username")
}

func generateRandomPassword(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[i] = chars[n.Int64()]
	}
	return string(b), nil
}

func runUserAdd(cmd *cobra.Command, _ []string) error {
	username, _ := cmd.Flags().GetString("username")
	email, _ := cmd.Flags().GetString("email")
	groupsStr, _ := cmd.Flags().GetString("groups")
	provider, _ := cmd.Flags().GetString("provider")
	usersFile, _ := cmd.Flags().GetString("users-file")
	passwordStdin, _ := cmd.Flags().GetBool("password-stdin")
	createOTSLink, _ := cmd.Flags().GetBool("create-ots-link")
	otsURL, _ := cmd.Flags().GetString("ots-url")

	if provider == "" {
		provider = "local"
	}

	groups := make([]string, 0)
	if groupsStr != "" {
		for _, g := range strings.Split(groupsStr, ",") {
			clean := strings.TrimSpace(g)
			if clean != "" {
				groups = append(groups, clean)
			}
		}
	}

	var password string
	var hash string
	var err error

	if provider == "local" {
		if passwordStdin {
			inputBytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read password from stdin: %w", err)
			}
			password = strings.TrimSpace(string(inputBytes))
			if password == "" {
				return fmt.Errorf("password read from stdin was empty")
			}
		} else {
			password, err = generateRandomPassword(32)
			if err != nil {
				return fmt.Errorf("failed to generate random password: %w", err)
			}
		}

		hash, err = auth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
	}

	la, err := auth.NewLocalAuthenticator(usersFile)
	if err != nil {
		return fmt.Errorf("failed to load user authenticator: %w", err)
	}

	record := auth.UserRecord{
		Username:  username,
		Provider:  provider,
		Hash:      hash,
		Email:     email,
		Groups:    groups,
		Disabled:  false,
		CreatedAt: time.Now(),
	}

	if err := la.AddUser(record); err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}

	if err := la.SaveUsers(); err != nil {
		return fmt.Errorf("failed to save users file '%s': %w", usersFile, err)
	}

	fmt.Printf("=========================================================================\n")
	fmt.Printf(" SUCCESS: User '%s' provisioned in %s\n", username, usersFile)
	fmt.Printf("=========================================================================\n")
	if provider == "local" && password != "" {
		fmt.Printf(" Generated Password:  %s\n", password)
		fmt.Printf("-------------------------------------------------------------------------\n")
		fmt.Printf(" ONBOARDING MESSAGE TEMPLATE:\n")
		fmt.Printf(" Hello %s,\n", username)
		fmt.Printf(" Your OTS account has been created.\n")
		fmt.Printf(" Username: %s\n", username)
		fmt.Printf(" Password: %s\n", password)
		fmt.Printf("-------------------------------------------------------------------------\n")

		if createOTSLink {
			onboardingText := fmt.Sprintf("Hello %s,\nYour OTS account has been created.\nUsername: %s\nPassword: %s", username, username, password)
			secretURL, expiresAt, err := client.Create(otsURL, client.Secret{Secret: onboardingText}, 0)
			if err != nil {
				logrus.WithError(err).Warn("Failed to create OTS onboarding link")
			} else {
				fmt.Printf(" Single-Use OTS Secret Link Generated (Expires: %s):\n", expiresAt.Format(time.RFC3339))
				fmt.Printf(" %s\n", secretURL)
				fmt.Printf(" (Share this single-use link with %s. It burns after one read!)\n", username)
				fmt.Printf("-------------------------------------------------------------------------\n")
			}
		}
	}

	return nil
}

func runUserList(cmd *cobra.Command, _ []string) error {
	usersFile, _ := cmd.Flags().GetString("users-file")

	la, err := auth.NewLocalAuthenticator(usersFile)
	if err != nil {
		return fmt.Errorf("failed to load user authenticator: %w", err)
	}

	users := la.ListUsers()
	fmt.Printf("=========================================================================\n")
	fmt.Printf(" PROVISIONED USERS (%s)\n", usersFile)
	fmt.Printf("=========================================================================\n")
	if len(users) == 0 {
		fmt.Println(" (No user accounts found)")
		return nil
	}

	for _, u := range users {
		status := "Active"
		if u.Disabled {
			status = "Disabled"
		}
		fmt.Printf(" Username: %s | Provider: %s | Status: %s | Groups: [%s]\n",
			u.Username, u.Provider, status, strings.Join(u.Groups, ", "))
	}
	fmt.Printf("=========================================================================\n")

	return nil
}

func runUserDisable(cmd *cobra.Command, _ []string) error {
	username, _ := cmd.Flags().GetString("username")
	usersFile, _ := cmd.Flags().GetString("users-file")

	la, err := auth.NewLocalAuthenticator(usersFile)
	if err != nil {
		return fmt.Errorf("failed to load user authenticator: %w", err)
	}

	users := la.ListUsers()
	found := false
	for _, u := range users {
		if strings.EqualFold(u.Username, username) {
			u.Disabled = true
			_ = la.AddUser(u)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("user '%s' not found", username)
	}

	if err := la.SaveUsers(); err != nil {
		return fmt.Errorf("failed to save users file: %w", err)
	}

	fmt.Printf("User '%s' has been disabled.\n", username)
	return nil
}

func runUserDelete(cmd *cobra.Command, _ []string) error {
	username, _ := cmd.Flags().GetString("username")
	usersFile, _ := cmd.Flags().GetString("users-file")

	la, err := auth.NewLocalAuthenticator(usersFile)
	if err != nil {
		return fmt.Errorf("failed to load user authenticator: %w", err)
	}

	if err := la.DeleteUser(username); err != nil {
		return fmt.Errorf("failed to delete user '%s': %w", username, err)
	}

	fmt.Printf("User '%s' deleted successfully.\n", username)
	return nil
}
