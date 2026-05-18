package clickhouse

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	conn driver.Conn
	once sync.Once
)

// InitClickHouse initializes the ClickHouse connection
func InitClickHouse(addr, db, user, pass string) (driver.Conn, error) {
	var err error
	once.Do(func() {
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{addr},
			Auth: clickhouse.Auth{
				Database: db,
				Username: user,
				Password: pass,
			},
			Settings: clickhouse.Settings{
				"max_execution_time": 60,
			},
			DialTimeout: time.Second * 30,
			Compression: &clickhouse.Compression{
				Method: clickhouse.CompressionLZ4,
			},
		})

		if err != nil {
			return
		}

		if err = conn.Ping(context.Background()); err != nil {
			return
		}

		log.Println("ClickHouse connection initialized successfully")
	})

	return conn, err
}

// GetConn returns the initialized ClickHouse connection
func GetConn() driver.Conn {
	if conn == nil {
		log.Fatal("ClickHouse connection has not been initialized.")
	}
	return conn
}
