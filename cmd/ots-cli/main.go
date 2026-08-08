// The OTS Command Line Interface Entrypoint
//
// Objectives:
// - Provides a lightweight command line utility for interacting with OTS secret sharing instances.
// - Supports secret creation, fetching, burning, user provisioning, and instance inspection.
// - Returns granular exit codes for automated CI/CD pipeline diagnostics.
//
// Core Components:
// - main: CLI entrypoint evaluating Cobra command execution errors and mapping them to granular exit codes.
// - Exit Codes: ExitSuccess (0), ExitGeneralError (1), ExitInvalidArgs (2), ExitNetworkError (3), ExitSecretNotFound (4), ExitDecryptionFailed (5).
//
// Data Flow:
// 1. User executes `ots-cli <command>` -> Cobra parses flags and positional arguments.
// 2. Invokes Go Client SDK (`pkg/client`) -> Communicates via REST API to OTS server.
// 3. Output rendered to stdout/stderr -> Process terminates with exit code.
package main

import (
	"errors"
	"os"
	"strings"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

const (
	ExitSuccess          = 0
	ExitGeneralError     = 1
	ExitInvalidArgs      = 2
	ExitNetworkError     = 3
	ExitSecretNotFound   = 4
	ExitDecryptionFailed = 5
)

var version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		errStr := strings.ToLower(err.Error())
		switch {
		case errors.Is(err, storage.ErrSecretNotFound) || strings.Contains(errStr, "404") || strings.Contains(errStr, "not found"):
			os.Exit(ExitSecretNotFound)
		case strings.Contains(errStr, "decrypt") || strings.Contains(errStr, "password"):
			os.Exit(ExitDecryptionFailed)
		case strings.Contains(errStr, "argument") || strings.Contains(errStr, "url"):
			os.Exit(ExitInvalidArgs)
		case strings.Contains(errStr, "connection") || strings.Contains(errStr, "dial") || strings.Contains(errStr, "timeout"):
			os.Exit(ExitNetworkError)
		default:
			os.Exit(ExitGeneralError)
		}
	}
}
