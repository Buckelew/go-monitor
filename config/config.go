package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type (
	Config struct {
		App      App
		Database Database
		Discord  Discord
	}

	App struct {
		LogLevel string `env:"LOG_LEVEL,required"`
		Env      string `env:"ENV,required"`
	}

	Database struct {
		Host       string `env:"POSTGRES_HOST,required"`
		Port       string `env:"POSTGRES_PORT,required"`
		DBName     string `env:"POSTGRES_DB_NAME,required"`
		TestDBName string `env:"POSTGRES_TEST_DB_NAME,required"`
		User       string `env:"POSTGRES_USER,required"`
		Password   string `env:"POSTGRES_PASSWORD,required"`
		URL        string `env:"POSTGRES_URL,required"`
		TestURL    string `env:"POSTGRES_TEST_URL,required"`
	}

	Discord struct {
		Token string `env:"DISCORD_TOKEN,required"`
	}
)

func NewConfig() (*Config, error) {
	config := &Config{}
	if err := env.Parse(config); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return config, nil
}
