package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	"forge/internal/config"
	"forge/internal/migrations"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	if err := migrations.Run(ctx, conn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	log.Println("migrations completed successfully")
}
