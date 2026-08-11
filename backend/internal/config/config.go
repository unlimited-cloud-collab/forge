package config

import "os"

const defaultPort = "8080"

const defaultDatabaseURL = "postgres://forge:forge_dev_password@localhost:5432/forge"

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() Config {
	port := os.Getenv("PORT")

	if port == "" {
		port = defaultPort
	}

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}

	return Config{
		Port:        port,
		DatabaseURL: databaseURL,
	}
}
