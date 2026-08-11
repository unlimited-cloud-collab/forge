package database

import "context"

// Pinger represents the database operation needed by readiness checks.
type Pinger interface {
	Ping(context.Context) error
}
