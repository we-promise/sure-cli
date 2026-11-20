package root

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
)

func newRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh OAuth access token using stored refresh token",
		Run: func(cmd *cobra.Command, args []string) {
			rt := config.RefreshToken()
			if rt == "" {
				output.Fail(errs.CodeRefreshTokenMissing, "no refresh token stored; run sure-cli login", nil)
			}

			client := api.New()
			res, err := client.Refresh(api.RefreshRequest{
				RefreshToken: rt,
				Device:       config.Device(),
			})
			if err != nil {
				ce := errs.From(err)
				output.Fail(errs.CodeRefreshFailed, ce.Message, ce.Details)
			}
			if res.AccessToken == "" {
				output.Fail(errs.CodeRefreshFailed, "refresh response returned empty access token", nil)
			}

			config.SetAuthMode("bearer")
			config.SetToken(res.AccessToken)
			// Guard rotation fields so a partial server response doesn't wipe
			// the saved values and silently log the user out.
			if res.RefreshToken != "" {
				config.SetRefreshToken(res.RefreshToken)
			}
			if res.ExpiresIn > 0 {
				config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
			}
			if err := config.Save(); err != nil {
				output.Fail(errs.CodeConfigSaveFailed, err.Error(), nil)
			}

			printEnvelope(map[string]any{
				"token_type": res.TokenType,
				"expires_in": res.ExpiresIn,
				"created_at": res.CreatedAt,
			}, nil)
		},
	}

	return cmd
}
