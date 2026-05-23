package root

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/config"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newLoginCmd() *cobra.Command {
	var email string
	var otp string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login via OAuth (email/password) and store access + refresh tokens",
		Run: func(cmd *cobra.Command, args []string) {
			// Prompt for email if not provided
			if email == "" {
				fmt.Print("Email: ")
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				email = strings.TrimSpace(input)
			}

			// Read password without echo when stdin is a TTY; fall back to a
			// plain line read when it's piped (e.g. CI smoke tests). The
			// piped path never echoes the password back to argv/ps/history.
			var password string
			if term.IsTerminal(int(syscall.Stdin)) {
				fmt.Print("Password: ")
				passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println() // newline after hidden input
				if err != nil {
					output.Fail("password_read_failed", err.Error(), nil)
					return
				}
				password = string(passwordBytes)
			} else {
				reader := bufio.NewReader(os.Stdin)
				input, err := reader.ReadString('\n')
				if err != nil && input == "" {
					output.Fail("password_read_failed", err.Error(), nil)
					return
				}
				password = strings.TrimRight(input, "\r\n")
			}

			client := api.New()

			res, err := client.Login(api.LoginRequest{
				Email:    email,
				Password: password,
				OTPCode:  otp,
				Device:   config.Device(),
			})
			if err != nil {
				output.Fail("login_failed", err.Error(), nil)
				return
			}
			if res.AccessToken == "" {
				output.Fail("login_failed", "login response returned empty access token", nil)
				return
			}

			config.SetAuthMode("bearer")
			config.SetToken(res.AccessToken)
			// Guard rotation fields so a partial server response doesn't wipe
			// saved tokens and silently log the user out.
			if res.RefreshToken != "" {
				config.SetRefreshToken(res.RefreshToken)
			}
			if res.ExpiresIn > 0 {
				config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
			}
			if err := config.Save(); err != nil {
				output.Fail("config_save_failed", err.Error(), nil)
				return
			}

			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"token_type": res.TokenType,
				"expires_in": res.ExpiresIn,
				"created_at": res.CreatedAt,
				"user":       res.User,
			}})
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "user email (prompted if not provided)")
	cmd.Flags().StringVar(&otp, "otp", "", "OTP code (if 2FA enabled)")
	return cmd
}
