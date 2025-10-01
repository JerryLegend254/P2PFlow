package main

import (
	"os"

	"github.com/JerryLegend254/p2pflow/internal/auth"
	"github.com/JerryLegend254/p2pflow/internal/env"
	"github.com/JerryLegend254/p2pflow/internal/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func main() {
	cfg := config{
		auth: authConfig{
			oauth: oauthConfig{
				config: &oauth2.Config{
					ClientID:     env.GetString("GITHUB_CLIENT_ID", "Ov23liC3LuwooH4KTcjX"),
					ClientSecret: env.GetString("GITHUB_CLIENT_SECRET", "2acc8a37e834ed000159dab091a6964e591a61e1"),
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
		config:  cfg,
		auth:    authenticator,
		logger:  jsonLogger,
		console: consoleLogger,
	}

	// register commands
	app.registerCommand(app.newLoginCommand(app.config.auth.oauth.config))

	if err := app.run(rootCmd); err != nil {
		app.console.Error(err.Error())
		os.Exit(1)
	}
}
