package main

import (
	"os"
	"path/filepath"

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
	path, _ := app.configPath()
	app.console.Info(path)
	var cfg appConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err

	}
	return &cfg, nil
}
