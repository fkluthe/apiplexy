package backend

import (
	"errors"
	"fmt"
	"github.com/12foo/apiplexy/conventions"
	"github.com/12foo/apiplexy/helpers"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"time"
)

var defaults = map[string]interface{}{
	"driver":   "",
	"host":     "",
	"port":     0,
	"database": "",
	"user":     "",
	"password": "",
	"ssl":      "",
}

type dbKey struct {
	conventions.Key
	UserID    string
	CreatedAt time.Time
	DeletedAt *time.Time
}

type dbUser struct {
	conventions.User
	CreatedAt time.Time
	DeletedAt *time.Time
}

type sqlDBBackend struct {
	db *gorm.DB
}

func (sql *sqlDBBackend) GetKey(keyId string, keyType string) (*conventions.Key, error) {
	k := &dbKey{}
	if sql.db.First(&k, keyId).RecordNotFound() {
		return nil, nil
	}
	return &k.Key, nil
}

func (sql *sqlDBBackend) StoreKey(userID string, key *conventions.Key) error {
	return nil
}

// NewSQLDBBackend creates a backend plugin for popular SQL databases. It has the following
// configuration options (read from your config JSON):
//
//  driver: one of "mysql", "postgres", "mssql", "sqlite3"
//  host: "localhost"
//  port: 3306
//  database: "my_db_name"
//  user: "my_user"
//  password: "my_password"
//  ssl: true (will use SSL if available with your database)
//
// The backend plugin supports full user management. Tables APIUser and APIKey will be created
// if they don't already exist. Records in these tables are 'soft-deleted', so they remain
// available after deletion in case you need to investigate API keys after the fact.
func NewSQLDBBackend(config map[string]interface{}) (interface{}, error) {
	if err := helpers.EnsureDefaults(config, defaults); err != nil {
		return nil, err
	}
	driverName := config["driver"]
	if driverName != "mysql" && driverName != "postgres" && driverName != "mssql" && driverName == "sqlite3" {
		return nil, errors.New(fmt.Sprintf("'%s' is not a valid driver for a SQL DB.", driverName))
	}

	return &sqlDBBackend{}, nil
}
