package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type appConfig struct {
	Auth struct {
		Token     string `mapstructure:"token"`
		Username  string `mapstructure:"username"`
		ExpiresAt string `mapstructure:"expires_at"`
		Name      string `mapstructure:"name"`
		Provider  string `mapstructure:"provider"`
	} `mapstructure:"auth"`
	Ignore struct {
		Patterns       []string `mapstructure:"patterns"`
		UseDefaults    bool     `mapstructure:"use_defaults"`
		UseP2PIgnore   bool     `mapstructure:"use_p2pignore"`
	} `mapstructure:"ignore"`
}

func (app *application) initConfig(cfgFile string) error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		configPath, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		appConfigDir := filepath.Join(configPath, app.appName)
		if _, err := os.Stat(appConfigDir); os.IsNotExist(err) {
			os.MkdirAll(appConfigDir, 0755)
		}

		viper.AddConfigPath(appConfigDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	// Set default values for ignore configuration
	viper.SetDefault("ignore.use_defaults", true)
	viper.SetDefault("ignore.use_p2pignore", true)
	viper.SetDefault("ignore.patterns", []string{})

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// TODO: Add default config
			app.saveConfig()
		} else {
			return err
		}
	}

	return nil
}

func (app *application) saveConfig() {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		dir, _ := os.UserConfigDir()
		configPath = filepath.Join(dir, app.appName, "config.yaml")
	}
	viper.WriteConfigAs(configPath)
}

func (app *application) loadAuth() (*appConfig, error) {
	var cfg appConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err

	}
	return &cfg, nil
}

func (app *application) newConfigSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config:set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			viper.Set(key, value)
			app.saveConfig()
			app.console.Infof("Updated %s = %s\n", key, value)
			return nil
		},
	}

	return cmd
}

func (app *application) newConfiShowCommand() *cobra.Command {
	cmd := &cobra.Command{}

	return cmd
}
