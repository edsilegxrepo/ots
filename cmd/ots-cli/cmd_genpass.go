package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const defaultPassLength = 32

var genpassCmd = &cobra.Command{
	Use:     "genpass",
	Aliases: []string{"generate-password", "passwd"},
	Short:   "Generate a secure cryptographically random password for secrets",
	Example: `  ots-cli genpass
  ots-cli genpass --length 64`,
	Args: cobra.NoArgs,
	RunE: genpassRunE,
}

func init() {
	genpassCmd.Flags().IntP("length", "l", defaultPassLength, "Length of the generated password")
	rootCmd.AddCommand(genpassCmd)
}

func genpassRunE(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	length, err := cmd.Flags().GetInt("length")
	if err != nil || length <= 0 {
		length = defaultPassLength
	}

	pass, err := generateRandomPassword(length)
	if err != nil {
		return fmt.Errorf("generating password: %w", err)
	}

	fmt.Println(pass)
	return nil
}
