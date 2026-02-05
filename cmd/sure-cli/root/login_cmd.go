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

			// Always prompt for password (hidden input)
			fmt.Print("Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println() // newline after hidden input
			if err != nil {
				output.Fail("password_read_failed", err.Error(), nil)
			}
			password := string(passwordBytes)

			client := api.New()

			res, err := client.Login(api.LoginRequest{
				Email:    email,
				Password: password,
				OTPCode:  otp,
				Device:   config.Device(),
			})
			if err != nil {
				output.Fail("login_failed", err.Error(), nil)
			}

			config.SetAuthMode("bearer")
			config.SetToken(res.AccessToken)
			config.SetRefreshToken(res.RefreshToken)
			config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
			if err := config.Save(); err != nil {
				output.Fail("config_save_failed", err.Error(), nil)
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
