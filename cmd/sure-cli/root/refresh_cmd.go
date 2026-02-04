package root

import (
	"time"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/config"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh OAuth access token using stored refresh token",
		Run: func(cmd *cobra.Command, args []string) {
			rt := config.RefreshToken()
			if rt == "" {
				output.Fail("missing_refresh_token", "no refresh token stored; run sure-cli login", nil)
			}

			client := api.New()
			res, err := client.Refresh(api.RefreshRequest{
				RefreshToken: rt,
				Device:       config.Device(),
			})
			if err != nil {
				output.Fail("refresh_failed", err.Error(), nil)
			}

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
			}})
		},
	}

	return cmd
}
