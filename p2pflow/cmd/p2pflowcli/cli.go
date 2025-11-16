package main

import (
	"github.com/JerryLegend254/p2pflow/internal/auth"
	"github.com/JerryLegend254/p2pflow/internal/logger"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "p2pflow",
	Short:   "P2P file synchronization with intelligent features",
	Version: version,
	Long:    "P2PFlow enables real-time peer-to-peer file synchronization for development teams.",
}

type application struct {
	appName string
	config  config
	auth    auth.Authenticator
	logger  *logger.Logger
	console *logger.Logger
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
	// config flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/p2pflow/config.yaml)")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return app.initConfig(cfgFile)
	}

	// register commands
	// auth commands
	rootCmd.AddCommand(app.newLoginCommand(app.config.auth.oauth.config))
	rootCmd.AddCommand(app.newWhoAmICommand())
	rootCmd.AddCommand(app.newLogOutCommand())

	// config commands
	rootCmd.AddCommand(app.newConfigSetCommand())
	rootCmd.AddCommand(app.newConfiShowCommand())

	// collab commands
	rootCmd.AddCommand(app.newCollabCommand())
	rootCmd.AddCommand(app.newCollabCRDTCommand())

	// analytics commands
	rootCmd.AddCommand(app.newAnalyticsCommand())
}

func (app *application) run(rootCmd *cobra.Command) error {
	// execute cobra cli
	return rootCmd.Execute()
}
