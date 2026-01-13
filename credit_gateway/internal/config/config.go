package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPPort      string
	DBHost        string
	DBPort        string
	DBName        string
	DBUser        string
	DBPass        string
	WebhookSecret string
}

func Load() (*Config, error) {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8083" // sensible default for local dev
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "credit_gateway"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "credit_gateway"
	}
	dbPass := os.Getenv("DB_PASS")
	if dbPass == "" {
		dbPass = "credit_gateway"
	}
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = "supersecret_1cent"
	}
	return &Config{
		HTTPPort:      port,
		DBHost:        dbHost,
		DBPort:        dbPort,
		DBName:        dbName,
		DBUser:        dbUser,
		DBPass:        dbPass,
		WebhookSecret: webhookSecret,
	}, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.HTTPPort)
}
func (c Config) WebhookSecretValue() string {
	return c.WebhookSecret
}

func (c *Config) PostgresDSN() string {
	// pgx format
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName,
	)
}
