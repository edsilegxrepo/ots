package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/edsilegxrepo/ots/pkg/client"
)

var rootCmd = &cobra.Command{
	Short:             "Utility to interact with encrypted secrets in an OTS instance",
	PersistentPreRunE: rootPersistentPreRunE,
}

func init() {
	rootCmd.PersistentFlags().String("log-level", "info", "Level to use for logging (trace, debug, info, warn, error, fatal)")
	rootCmd.PersistentFlags().Duration("timeout", 0, "HTTP request timeout for communicating with OTS instance (e.g. 15s, 1m)")
}

func rootPersistentPreRunE(cmd *cobra.Command, _ []string) error {
	sll, err := cmd.PersistentFlags().GetString("log-level")
	if err != nil {
		sll = "info"
	}

	ll, err := logrus.ParseLevel(sll)
	if err != nil {
		return fmt.Errorf("parsing log-level: %w", err)
	}
	logrus.SetLevel(ll)

	timeout, err := cmd.PersistentFlags().GetDuration("timeout")
	if err == nil && timeout > 0 {
		client.RequestTimeout = timeout
	}

	client.Logger = logrus.NewEntry(logrus.StandardLogger())

	return nil
}
