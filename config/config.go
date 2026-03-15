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
		Web      Web
		API      API
	}

	App struct {
		LogLevel string `env:"LOG_LEVEL,required"`
		Env      string `env:"ENV,required"`
	}

	Database struct {
		Host               string `env:"POSTGRES_HOST,required"`
		Port               string `env:"POSTGRES_PORT,required"`
		DBName             string `env:"POSTGRES_DB_NAME,required"`
		TestDBName         string `env:"POSTGRES_TEST_DB_NAME,required"`
		User               string `env:"POSTGRES_USER,required"`
		Password           string `env:"POSTGRES_PASSWORD,required"`
		URL                string `env:"POSTGRES_URL,required"`
		TestURL            string `env:"POSTGRES_TEST_URL,required"`
		MaxConnections     int    `env:"POSTGRES_MAX_CONNECTIONS" envDefault:"25"`
		MaxIdleConnections int    `env:"POSTGRES_MAX_IDLE_CONNECTIONS" envDefault:"5"`
	}

	Discord struct {
		Token        string `env:"DISCORD_TOKEN,required"`
		ClientID     string `env:"DISCORD_CLIENT_ID,required"`
		ClientSecret string `env:"DISCORD_CLIENT_SECRET,required"`
		RedirectURL  string `env:"DISCORD_REDIRECT_URL,required"`
	}

	Web struct {
		Port int `env:"WEB_PORT" envDefault:"8080"`
	}

	API struct {
		Port          int    `env:"API_PORT" envDefault:"3000"`
		SessionSecret string `env:"SESSION_SECRET,required"`
	}
)

func NewConfig() (*Config, error) {
	config := &Config{}
	if err := env.Parse(config); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return config, nil
}
