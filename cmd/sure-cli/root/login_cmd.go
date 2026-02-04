package root

import (
	"time"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/config"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var email string
	var password string
	var otp string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login via OAuth (email/password) and store access + refresh tokens",
		Run: func(cmd *cobra.Command, args []string) {
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

	cmd.Flags().StringVar(&email, "email", "", "user email")
	cmd.Flags().StringVar(&password, "password", "", "user password")
	cmd.Flags().StringVar(&otp, "otp", "", "OTP code (if 2FA enabled)")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}
