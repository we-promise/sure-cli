package root

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
	"golang.org/x/term"
)

func newLoginCmd() *cobra.Command {
	var email string
	var otp string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login via OAuth (email/password) and store access + refresh tokens",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			if err := client.Err(); err != nil {
				fail(errs.From(err))
			}
			requireWritableConfig()
			// Prompts go to stderr: stdout holds only the envelope. One
			// reader serves both lines, or the first would buffer the
			// password line when stdin is piped.
			stdin := bufio.NewReader(os.Stdin)
			if email == "" {
				fmt.Fprint(os.Stderr, "Email: ")
				input, _ := stdin.ReadString('\n')
				email = strings.TrimSpace(input)
			}

			// Read password without echo when stdin is a TTY; fall back to a
			// plain line read when it's piped (e.g. CI smoke tests). The
			// piped path never echoes the password back to argv/ps/history.
			var password string
			if term.IsTerminal(int(syscall.Stdin)) {
				fmt.Fprint(os.Stderr, "Password: ")
				passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Fprintln(os.Stderr) // newline after hidden input
				if err != nil {
					output.Fail(errs.CodePasswordReadFailed, err.Error(), nil)
				}
				password = string(passwordBytes)
			} else {
				input, err := stdin.ReadString('\n')
				if err != nil && input == "" {
					output.Fail(errs.CodePasswordReadFailed, err.Error(), nil)
				}
				password = strings.TrimRight(input, "\r\n")
			}

			// Every login revokes the device's previous tokens upstream, so
			// each install needs its own device_id (persisted below by Save).
			config.EnsureDeviceID()
			res, err := client.Login(api.LoginRequest{
				Email:    email,
				Password: password,
				OTPCode:  otp,
				Device:   config.Device(),
			})
			if err != nil {
				fail(loginError(err, otp))
			}
			if res.AccessToken == "" {
				output.Fail(errs.CodeLoginFailed, "login response returned empty access token", nil)
			}

			saveTokens(res.TokenResponse)

			printEnvelope(map[string]any{
				"token_type": res.TokenType,
				"expires_in": res.ExpiresIn,
				"created_at": res.CreatedAt,
				"user":       res.User,
			}, nil)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "user email (prompted if not provided)")
	cmd.Flags().StringVar(&otp, "otp", "", "OTP code (if 2FA enabled)")
	return cmd
}

// loginError maps a Login failure to the envelope error. Sure's MFA
// challenge (401 mfa_required) keeps its own code with a hint about --otp;
// Sure turning the credentials down is login_failed with Sure's message and
// details; any other failure keeps its own code (see credentialError).
func loginError(err error, otp string) *errs.CLIError {
	ce := errs.From(err)
	if ce.Code == errs.CodeMFARequired {
		msg := "Two-factor authentication required: re-run with --otp <code>"
		if otp != "" {
			msg = "Two-factor authentication failed: the --otp code was rejected"
		}
		return errs.New(errs.CodeMFARequired, msg).WithDetails(ce.Details)
	}
	return credentialError(errs.CodeLoginFailed, ce)
}

// credentialError is ce as code when Sure answered that it rejects the
// credentials or device a login or refresh sent (400, 401, 403, 422).
// Anything else (a redirect, a timeout, a rate limit, a 5xx, an answer lost
// or unreadable) keeps its own code, which says what to do about it.
func credentialError(code string, ce *errs.CLIError) *errs.CLIError {
	switch ce.Details["status"] {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity:
		return errs.New(code, ce.Message).WithDetails(ce.Details)
	}
	return ce
}

// requireWritableConfig fails before a call that makes Sure revoke the stored
// tokens (a login revokes the device's, a refresh the refresh token), so
// tokens the CLI could not save do not log the user out.
func requireWritableConfig() {
	if err := config.CheckWritable(); err != nil {
		fail(errs.Wrap(errs.CodeConfigSaveFailed, fmt.Sprintf("config file is not writable (%v); nothing was sent to Sure", err), err))
	}
}

// saveTokens stores the tokens of a login or refresh. If the config cannot
// be saved the new pair is rescued (see config.RescueTokens).
func saveTokens(res api.TokenResponse) {
	config.SetAuthMode("bearer")
	config.SetToken(res.AccessToken)
	// Guard rotation fields so a partial server response doesn't wipe saved
	// tokens and silently log the user out.
	if res.RefreshToken != "" {
		config.SetRefreshToken(res.RefreshToken)
	}
	if res.ExpiresIn > 0 {
		config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
	}
	if err := config.Save(); err != nil {
		fail(config.RescueTokens(err))
	}
}
