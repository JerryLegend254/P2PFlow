package main

import (
	"os"

	"github.com/JerryLegend254/p2pflow/internal/auth"
	"github.com/JerryLegend254/p2pflow/internal/env"
	"github.com/JerryLegend254/p2pflow/internal/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var version = "0.0.0"

func main() {
	cfg := config{
		auth: authConfig{
			oauth: oauthConfig{
				config: &oauth2.Config{
					ClientID:     env.GetString("GITHUB_CLIENT_ID", ""),
					ClientSecret: env.GetString("GITHUB_CLIENT_SECRET", ""),
					Endpoint:     github.Endpoint,
					Scopes:       []string{"read:user", "user:email"},
				},
				state: auth.GenerateRandomState(),
			},
		},
	}

	jsonLogger := logger.NewLogger(logger.JSON)
	consoleLogger := logger.NewLogger(logger.CONSOLE)

	defer jsonLogger.Sync()
	defer consoleLogger.Sync()

	authenticator := auth.NewAuthenticator(cfg.auth.oauth.config, cfg.auth.oauth.state)

	app := &application{
		appName: "p2pflow",
		config:  cfg,
		auth:    authenticator,
		logger:  jsonLogger,
		console: consoleLogger,
	}

	// mount root command
	app.mount()

	if err := app.run(rootCmd); err != nil {
		app.console.Errorf("Erorr encountered %v", err.Error())
		os.Exit(1)
	}
}
