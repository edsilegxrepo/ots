package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/edsilegxrepo/ots/pkg/client"
)

var infoCmd = &cobra.Command{
	Use:     "info [instance]",
	Aliases: []string{"settings", "status"},
	Short:   "Display server settings, limits, and allowed file extensions for an OTS instance",
	Example: `  ots-cli info
  ots-cli info http://127.0.0.1:3000/
  ots-cli info --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: infoRunE,
}

func init() {
	defaultInstance := "http://127.0.0.1:3000/"
	if inst := os.Getenv("OTS_INSTANCE"); inst != "" {
		defaultInstance = inst
	}

	infoCmd.Flags().String("instance", defaultInstance, "Instance to query settings for")
	infoCmd.Flags().Bool("json", false, "Output settings in JSON format")
	rootCmd.AddCommand(infoCmd)
}

func infoRunE(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	instanceURL, _ := cmd.Flags().GetString("instance")
	if len(args) > 0 && args[0] != "" {
		instanceURL = args[0]
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")

	logrus.WithField("instance", instanceURL).Info("fetching server settings...")
	settings, err := client.LoadSettings(instanceURL)
	if err != nil {
		return fmt.Errorf("loading settings for %s: %w", instanceURL, err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(settings)
	}

	title := settings.AppTitle
	if title == "" {
		title = "One-Time-Secret (OTS)"
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" OTS INSTANCE INFORMATION (%s)\n", instanceURL)
	fmt.Println("=========================================================================")
	fmt.Printf(" App Title                 : %s\n", title)
	fmt.Printf(" Attachments Enabled       : %t\n", !settings.DisableFileAttachment)
	fmt.Printf(" Max Attachment Size Total : %d bytes\n", settings.MaxAttachmentSizeTotal)
	fmt.Printf(" Max Secret Reads          : %d\n", settings.MaxSecretReads)

	exts := settings.ResolvedAcceptedExtensions
	if len(exts) == 0 && settings.AcceptedFileTypes != "" {
		exts = []string{settings.AcceptedFileTypes}
	}
	if len(exts) == 0 {
		fmt.Printf(" Allowed File Extensions   : ALL (*)\n")
	} else {
		fmt.Printf(" Allowed File Extensions   : %s\n", strings.Join(exts, ", "))
	}
	fmt.Println("=========================================================================")

	return nil
}
