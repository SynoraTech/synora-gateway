package pg

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool *pgxpool.Pool
	once sync.Once
)

// InitDB initializes the PostgreSQL connection pool
func InitDB(connString string) (*pgxpool.Pool, error) {
	var err error
	once.Do(func() {
		config, parseErr := pgxpool.ParseConfig(connString)
		if parseErr != nil {
			err = fmt.Errorf("unable to parse connection string: %v", parseErr)
			return
		}

		// Configure pool settings if needed
		config.MaxConns = 20

		pool, err = pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			err = fmt.Errorf("unable to create connection pool: %v", err)
			return
		}

		// Ping the database to ensure connection
		if pingErr := pool.Ping(context.Background()); pingErr != nil {
			err = fmt.Errorf("unable to ping database: %v", pingErr)
			return
		}

		log.Println("PostgreSQL connection pool initialized successfully")
	})

	return pool, err
}

// GetDB returns the initialized connection pool
func GetDB() *pgxpool.Pool {
	if pool == nil {
		log.Fatal("PostgreSQL pool has not been initialized. Call InitDB first.")
	}
	return pool
}

// SetDBForTest sets the DB pool for testing
func SetDBForTest(p *pgxpool.Pool) {
	pool = p
}

// CloseDB closes the connection pool
func CloseDB() {
	if pool != nil {
		pool.Close()
	}
}
