package main

import (
	"bufio"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/edsilegxrepo/ots/pkg/client"
)

type (
	authRoundTripper struct {
		http.RoundTripper

		headers    http.Header
		user, pass string
	}
)

var createCmd = &cobra.Command{
	Use:   "create [note] [-n note] [-f file]... [--instance url] [--secret-from file]",
	Short: "Create a new encrypted secret note in the given OTS instance",
	Long:  "Creates an encrypted secret note or file attachment on an OTS instance. Content can be supplied via argument, --note flag, stdin, or file.",
	Example: `  ots-cli create "My secret password or note"
  ots-cli create -n "My secret note"
  echo "I'm a very secret note" | ots-cli create
  ots-cli create -f confidential.pdf`,
	Args: cobra.MaximumNArgs(1),
	RunE: createRunE,
}

func init() {
	defaultInstance := "http://127.0.0.1:3000/"
	if inst := os.Getenv("OTS_INSTANCE"); inst != "" {
		defaultInstance = inst
	}

	createCmd.Flags().Duration("expire", 0, "When to expire the secret (0 to use server-default)")
	createCmd.Flags().StringSliceP("header", "H", nil, "Headers to include in the request (i.e. 'Authorization: Token ...')")
	createCmd.Flags().String("instance", defaultInstance, "Instance to create the secret with")
	createCmd.Flags().StringSliceP("file", "f", nil, "File(s) to attach to the secret")
	createCmd.Flags().StringP("note", "n", "", "Secret / note text content to encrypt")
	createCmd.Flags().Bool("no-text", false, "Disable secret read (create a secret with only files)")
	createCmd.Flags().IntP("reads", "r", 1, "Number of allowed views before secret is permanently destroyed")
	createCmd.Flags().String("secret-from", "-", `File to read the secret content from ("-" for STDIN)`)
	createCmd.Flags().StringP("user", "u", "", "Username / Password for basic auth, specified as 'user:pass'")
	rootCmd.AddCommand(createCmd)
}

func createRunE(cmd *cobra.Command, args []string) (err error) {
	cmd.SilenceUsage = true

	var secret client.Secret

	if client.HTTPClient, err = constructHTTPClient(cmd); err != nil {
		return fmt.Errorf("constructing authorized HTTP client: %w", err)
	}

	// Read the secret content
	logrus.Info("reading secret content...")
	if secret.Secret, err = getSecretContent(cmd, args); err != nil {
		return fmt.Errorf("getting secret content: %w", err)
	}

	// Attach any file given
	files, err := cmd.Flags().GetStringSlice("file")
	if err != nil {
		return fmt.Errorf("getting file flag: %w", err)
	}
	for _, f := range files {
		logrus.WithField("file", f).Info("attaching file...")
		content, err := os.ReadFile(f) //#nosec:G304 // Opening user specified file is intended
		if err != nil {
			return fmt.Errorf("reading attachment %q: %w", f, err)
		}

		secret.Attachments = append(secret.Attachments, client.SecretAttachment{
			Name:    path.Base(f),
			Type:    mime.TypeByExtension(path.Ext(f)),
			Content: content,
		})
	}

	if secret.Secret == "" && secret.Attachments == nil {
		return fmt.Errorf("secret has no content and no attachments")
	}

	// Get flags for creation
	logrus.Info("creating the secret...")
	instanceURL, err := cmd.Flags().GetString("instance")
	if err != nil {
		return fmt.Errorf("getting instance flag: %w", err)
	}

	expire, err := cmd.Flags().GetDuration("expire")
	if err != nil {
		return fmt.Errorf("getting expire flag: %w", err)
	}

	reads, err := cmd.Flags().GetInt("reads")
	if err != nil {
		return fmt.Errorf("getting reads flag: %w", err)
	}

	// Execute sanity checks
	if err = client.SanityCheck(instanceURL, secret); err != nil {
		return fmt.Errorf("sanity checking secret: %w", err)
	}

	// Create the secret
	secretURL, expiresAt, err := client.CreateWithOpts(instanceURL, secret, client.CreateOpts{
		ExpireIn: expire,
		Reads:    reads,
	})
	if err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}

	// Tell them where to find the secret
	if expiresAt.IsZero() {
		logrus.Info("secret created, see URL below")
	} else {
		logrus.WithField("expires-at", expiresAt).Info("secret created, see URL below")
	}
	fmt.Println(secretURL) //nolint:forbidigo // Output intended for STDOUT

	return nil
}

func constructHTTPClient(cmd *cobra.Command) (*http.Client, error) {
	basic, _ := cmd.Flags().GetString("user")
	headers, _ := cmd.Flags().GetStringSlice("header")

	if basic == "" && headers == nil {
		// No authorization needed
		return http.DefaultClient, nil
	}

	t := authRoundTripper{RoundTripper: http.DefaultTransport, headers: http.Header{}}

	// Set basic auth if available
	user, pass, ok := strings.Cut(basic, ":")
	if ok {
		t.user = user
		t.pass = pass
	}

	// Parse and set headers if available
	for _, hdr := range headers {
		key, value, ok := strings.Cut(hdr, ":")
		if !ok {
			logrus.WithField("header", hdr).Warn("invalid header format, skipping")
			continue
		}
		t.headers.Add(key, strings.TrimSpace(value))
	}

	return &http.Client{Transport: t}, nil
}

func getSecretContent(cmd *cobra.Command, args []string) (string, error) {
	noteFlag, err := cmd.Flags().GetString("note")
	if err != nil {
		return "", fmt.Errorf("getting note flag: %w", err)
	}
	if noteFlag != "" {
		return noteFlag, nil
	}

	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}

	noSecret, err := cmd.Flags().GetBool("no-text")
	if err != nil {
		return "", fmt.Errorf("getting no-text flag: %w", err)
	}
	if noSecret {
		return "", nil
	}

	secretSourceName, err := cmd.Flags().GetString("secret-from")
	if err != nil {
		return "", fmt.Errorf("getting secret-from flag: %w", err)
	}

	if secretSourceName != "" && secretSourceName != "-" {
		f, err := os.Open(secretSourceName) //#nosec:G304 // Opening user specified file is intended
		if err != nil {
			return "", fmt.Errorf("opening secret-from file: %w", err)
		}
		defer f.Close() //nolint:errcheck // Short-lived fd-leak
		secretContent, err := io.ReadAll(f)
		if err != nil {
			return "", fmt.Errorf("reading secret-from file: %w", err)
		}
		return strings.TrimSpace(string(secretContent)), nil
	}

	// Handled secret-from "-" (STDIN)
	stat, err := os.Stdin.Stat()
	isTerminal := (err == nil && (stat.Mode()&os.ModeCharDevice) != 0)

	files, _ := cmd.Flags().GetStringSlice("file")
	if isTerminal && len(files) > 0 && !cmd.Flags().Changed("secret-from") {
		// Terminal environment with attachment files and no explicit note argument/flag:
		// Default to file-only secret without blocking on terminal input.
		return "", nil
	}

	if isTerminal {
		fmt.Print("Enter secret note content: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return strings.TrimSpace(scanner.Text()), nil
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading interactive note input: %w", err)
		}
		return "", nil
	}

	// Non-terminal STDIN (e.g. piped input `echo "note" | ots-cli create`)
	secretContent, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading secret content from stdin: %w", err)
	}

	return strings.TrimSpace(string(secretContent)), nil
}

func (a authRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if a.user != "" {
		r.SetBasicAuth(a.user, a.pass)
	}

	for key, values := range a.headers {
		if r.Header == nil {
			r.Header = http.Header{}
		}
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}

	resp, err := a.RoundTripper.RoundTrip(r)
	if err != nil {
		return nil, fmt.Errorf("executing round-trip: %w", err)
	}
	return resp, nil
}
