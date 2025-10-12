package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type GetGithubUserResponse struct {
	Username  string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
	Name      string `json:"name"`
}

type AuthConfig struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func (app *application) newLoginCommand(conf *oauth2.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login into the CLI with GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Request device + user codes from GitHub
			form := url.Values{
				"client_id": {conf.ClientID},
				"scope":     conf.Scopes, // adjust scopes to your needs
			}
			req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/device/code", strings.NewReader(form.Encode()))
			if err != nil {
				return fmt.Errorf("error creating device code request: %w", err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("error getting device code: %w", err)
			}
			defer resp.Body.Close()

			var dc DeviceCodeResponse
			if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
				return fmt.Errorf("error decoding device code response: %w", err)
			}

			// Print instructions for the user
			app.console.Infof("Please visit %s and enter the code: %s", dc.VerificationURI, dc.UserCode)

			// Poll GitHub until authorized or expired
			interval := time.Duration(dc.Interval) * time.Second
			expiry := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

			for {
				if time.Now().After(expiry) {
					return fmt.Errorf("device code expired — please run login again")
				}

				tokenForm := url.Values{
					"client_id":   {conf.ClientID},
					"device_code": {dc.DeviceCode},
					"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				}
				tokenReq, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(tokenForm.Encode()))
				if err != nil {
					return fmt.Errorf("error creating token request: %w", err)
				}
				tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				tokenReq.Header.Set("Accept", "application/json")

				tokenResp, err := http.DefaultClient.Do(tokenReq)
				if err != nil {
					return fmt.Errorf("error exchanging device code: %w", err)
				}
				defer tokenResp.Body.Close()

				var tokenData map[string]any
				if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
					return fmt.Errorf("error decoding token response: %w", err)
				}

				// Handle errors or pending status
				if errStr, ok := tokenData["error"].(string); ok {
					switch errStr {
					case "authorization_pending":
						time.Sleep(interval)
						continue
					case "slow_down":
						interval += 2 * time.Second
						time.Sleep(interval)
						continue
					default:
						return fmt.Errorf("github returned error: %v", tokenData)
					}
				}

				// Success
				if accessToken, ok := tokenData["access_token"].(string); ok {
					ac := AuthConfig{Token: accessToken, ExpiresAt: time.Now().Add(24 * time.Hour * 30).Local().String()}

					// TODO: encrypt file
					req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
					if err != nil {
						return fmt.Errorf("error creating get user request: %w", err)
					}
					req.Header.Set("Accept", "application/json")
					req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ac.Token))

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return fmt.Errorf("error getting user: %w", err)
					}
					defer resp.Body.Close()

					var user GetGithubUserResponse
					if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
						return fmt.Errorf("error decoding user response: %w", err)
					}
					viper.Set("auth.provider", "github")
					viper.Set("auth.token", ac.Token)
					viper.Set("auth.username", user.Username)
					viper.Set("auth.name", user.Name)
					viper.Set("auth.expires_at", ac.ExpiresAt)

					app.saveConfig()
				} else {
					return fmt.Errorf("unexpected token response: %v", tokenData)
				}
				break
			}

			return nil
		},
	}

	return cmd
}

func (app *application) newWhoAmICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Check currently logged in user",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.loadAuth()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					app.console.Info("not logged in, use the login command to log in, by running:\np2pflow login")
					return nil
				}
				return fmt.Errorf("error loading details %v", err)
			}

			if cfg.Auth.Token == "" || cfg.Auth.Username == "" {
				app.console.Info("not logged in, use the login command to log in, by running:\np2pflow login")
				return nil
			}
			layout := "2006-01-02 15:04:05.99999 -0700 MST"
			expDate, err := time.Parse(layout, cfg.Auth.ExpiresAt)
			if err != nil {
				return fmt.Errorf("error parsing expiry date %s", err.Error())

			}
			if time.Now().After(expDate) {
				return fmt.Errorf("error token already expired, login again")

			}

			if len(cfg.Auth.Token) > 0 {
				app.console.Infof("You are logged in as: %s", cfg.Auth.Username)
			} else {

			}
			return nil
		},
	}
	return cmd

}

func (app *application) newLogOutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout currently logged in user",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := app.configPath()
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to logout %s", err.Error())

			}
			return nil
		},
	}
	return cmd

}

// TODO: update to config file passed just in case
func (app *application) configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, app.appName, "config.yaml"), nil
}
