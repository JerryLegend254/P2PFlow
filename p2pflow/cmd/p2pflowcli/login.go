package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
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

				var tokenData map[string]interface{}
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
						return fmt.Errorf("GitHub returned error: %v", tokenData)
					}
				}

				// Success
				if accessToken, ok := tokenData["access_token"].(string); ok {
					app.console.Info("Successfully obtained access token: ", accessToken)
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
