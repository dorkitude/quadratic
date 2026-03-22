package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const appName = "quadratic"

type Config struct {
	ConfigPath   string `mapstructure:"-"`
	DataDir      string `mapstructure:"data_dir"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
	AccessToken  string `mapstructure:"access_token"`
}

func Load() (*Config, error) {
	cfgPath, dataDirDefault, err := DefaultPaths()
	if err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("QUADRATIC")
	v.AutomaticEnv()

	v.SetDefault("data_dir", dataDirDefault)
	v.SetDefault("redirect_url", "http://127.0.0.1:8765/callback")

	if _, statErr := os.Stat(cfgPath); statErr == nil {
		if err := v.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("stat config: %w", statErr)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	cfg.ConfigPath = cfgPath
	return &cfg, nil
}

func DefaultPaths() (configPath string, dataDir string, err error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve config dir: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home dir: %w", err)
	}

	return filepath.Join(configDir, appName, "config.yaml"), filepath.Join(homeDir, "."+appName, "data"), nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	v := viper.New()
	v.Set("data_dir", cfg.DataDir)
	v.Set("client_id", cfg.ClientID)
	v.Set("client_secret", cfg.ClientSecret)
	v.Set("redirect_url", cfg.RedirectURL)
	v.Set("access_token", cfg.AccessToken)
	return v.WriteConfigAs(cfg.ConfigPath)
}
