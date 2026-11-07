// Package database opens registre's PostgreSQL connection.
package database

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Pool is the connection pool's size and age.
type Pool struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
}

// Open connects to the PostgreSQL database named by databaseURL with a quiet
// logger and translated errors, and caps the pool.
//
// The cap matters more than it looks: without it the pool opens a connection
// per concurrent request, and a login flood becomes "too many clients" for
// every other service on the same Postgres.
func Open(databaseURL string, pool Pool) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(pool.MaxOpen)
	sqlDB.SetMaxIdleConns(pool.MaxIdle)
	sqlDB.SetConnMaxLifetime(pool.MaxLifetime)
	return db, nil
}
