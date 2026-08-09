package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server   SeverConfig
	Database DatabaseConfig
}

type SeverConfig struct {
	Env          string `mapstructure:"ENV"`
	Port         string `mapstructure:"PORT"`
	ReadTimeOut  int    `mapstructure:"READ_TIME_OUT"`
	IdleTimeOut  int    `mapstructure:"IDLE_TIME_OUT"`
	WriteTimeOut int    `mapstructure:"WRITE_TIME_OUT"`
}

type DatabaseConfig struct {
	Host                  string `mapstructure:"DB_HOST"`
	Name                  string `mapstructure:"DB_NAME"`
	User                  string `mapstructure:"DB_USER"`
	Pass                  string `mapstructure:"DB_PASS"`
	Port                  string `mapstructure:"DB_PORT"`
	SSLMode               string `mapstructure:"DB_SSL_MODE"`
	MaxOpenConnection     int    `mapstructure:"DB_MAX_OPEN_CONNECTION"`
	MaxIdleConnection     int    `mapstructure:"DB_MAX_IDLE_CONNECTION"`
	MaxConnectionLifeTime int    `mapstructure:"DB_MAX_CONNECTION_LIFE_TIME"`
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
	cfg := &Config{
		Server: SeverConfig{
			Env:          viper.GetString("ENV"),
			Port:         viper.GetString("PORT"),
			IdleTimeOut:  viper.GetInt("IDLE_TIME_OUT"),
			WriteTimeOut: viper.GetInt("WRITE_TIME_OUT"),
			ReadTimeOut:  viper.GetInt("READ_TIME_OUT"),
		},
		Database: DatabaseConfig{
			Host:                  viper.GetString("DB_HOST"),
			Name:                  viper.GetString("DB_NAME"),
			User:                  viper.GetString("DB_USER"),
			Pass:                  viper.GetString("DB_PASS"),
			Port:                  viper.GetString("DB_PORT"),
			SSLMode:               viper.GetString("DB_SSL_MODE"),
			MaxOpenConnection:     viper.GetInt("DB_MAX_OPEN_CONNECTION"),
			MaxIdleConnection:     viper.GetInt("DB_MAX_IDLE_CONNECTION"),
			MaxConnectionLifeTime: viper.GetInt("DB_MAX_CONNECTION_LIFE_TIME"),
		},
	}
	cfg = setDefaultConfigs(cfg)
	err := validateConfigs(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func setDefaultConfigs(cfg *Config) *Config {
	if cfg.Server.Env == "" {
		cfg.Server.Env = "development"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.WriteTimeOut == 0 {
		cfg.Server.WriteTimeOut = 15
	}
	if cfg.Server.ReadTimeOut == 0 {
		cfg.Server.ReadTimeOut = 15
	}
	if cfg.Server.IdleTimeOut == 0 {
		cfg.Server.IdleTimeOut = 60
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}

	if cfg.Database.Port == "" {
		cfg.Database.Port = "8560"
	}
	if cfg.Database.MaxOpenConnection == 0 {
		cfg.Database.MaxOpenConnection = 100
	}
	if cfg.Database.MaxIdleConnection == 0 {
		cfg.Database.MaxIdleConnection = 10
	}
	if cfg.Database.MaxConnectionLifeTime == 0 {
		cfg.Database.MaxConnectionLifeTime = 1
	}
	return cfg
}

func validateConfigs(cfg *Config) error {

	if cfg.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if cfg.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if cfg.Database.Pass == "" {
		return fmt.Errorf("DB_PASS is required")
	}

	if cfg.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}
	return nil
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", d.Host, d.User, d.Pass, d.Name, d.Port, d.SSLMode)
}
