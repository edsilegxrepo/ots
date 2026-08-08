package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/edsilegxrepo/ots/pkg/client"
)

var burnCmd = &cobra.Command{
	Use:     "burn <url>",
	Aliases: []string{"destroy", "expire"},
	Short:   "Retrieves and permanently burns/destroys a secret from the OTS instance",
	Example: `  ots-cli burn http://127.0.0.1:3000/#secret-id|secret-key`,
	Args:    cobra.ExactArgs(1),
	RunE:    burnRunE,
}

func init() {
	rootCmd.AddCommand(burnCmd)
}

func burnRunE(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	secretURL := args[0]

	logrus.WithField("url", secretURL).Info("fetching secret to burn/destroy...")

	var err error
	if client.HTTPClient, err = constructHTTPClient(cmd); err != nil {
		return fmt.Errorf("constructing HTTP client: %w", err)
	}

	secret, err := client.Fetch(secretURL)
	if err != nil {
		return fmt.Errorf("burning secret: %w", err)
	}

	logrus.Info("secret retrieved and permanently burned from OTS instance")

	if secret.Secret != "" {
		fmt.Printf("Burned Secret Content: %s\n", secret.Secret)
	}
	if len(secret.Attachments) > 0 {
		fmt.Printf("Burned %d Attachment(s):\n", len(secret.Attachments))
		for _, a := range secret.Attachments {
			fmt.Printf("  - %s (%d bytes, type: %s)\n", a.Name, len(a.Content), a.Type)
		}
	}

	return nil
}
