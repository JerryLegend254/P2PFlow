package main

import (
	"github.com/JerryLegend254/p2pflow/internal/auth"
	"github.com/JerryLegend254/p2pflow/internal/logger"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var rootCmd = &cobra.Command{
	Use:   "p2pflow",
	Short: "P2P file synchronization with intelligent features",
	Long:  "P2PFlow enables real-time peer-to-peer file synchronization for development teams.",
}

type application struct {
	config  config
	auth    auth.Authenticator
	logger  logger.Logger
	console logger.Logger
}

type config struct {
	auth authConfig
}

type authConfig struct {
	oauth oauthConfig
}

type oauthConfig struct {
	config *oauth2.Config
	state  string
}

func (app *application) mount() {
	// register commands
	rootCmd.AddCommand(app.newLoginCommand(app.config.auth.oauth.config))
	rootCmd.AddCommand(app.newWhoAmICommand())
	rootCmd.AddCommand(app.newLogOutCommand())
}

func (app *application) run(rootCmd *cobra.Command) error {
	// execute cobra cli
	return rootCmd.Execute()
}
