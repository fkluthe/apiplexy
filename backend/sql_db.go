package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	c "github.com/12foo/apiplexy/conventions"
	"github.com/12foo/apiplexy/helpers"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
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

type sqlDBKey struct {
	ID        string
	Realm     string
	Type      string
	Data      string
	CreatedAt time.Time
	DeletedAt *time.Time
}

func (u *sqlDBKey) toKey() *c.Key {
	ck := c.Key{
		ID:    u.ID,
		Realm: u.Realm,
		Type:  u.Type,
	}
	json.Unmarshal([]byte(u.Data), &ck.Data)
	return &ck
}

type sqlDBUser struct {
	ID        string
	Email     string
	Name      string
	Password  []byte
	Admin     bool
	Active    bool
	Profile   []byte
	CreatedAt time.Time
	DeletedAt *time.Time
	LastLogin *time.Time
}

func (u *sqlDBUser) toUser() *c.User {
	cu := c.User{
		ID:     u.ID,
		Email:  u.Email,
		Name:   u.Name,
		Admin:  u.Admin,
		Active: u.Active,
	}
	json.Unmarshal(u.Profile, &cu.Profile)
	return &cu
}

type sqlDBBackend struct {
	db *gorm.DB
}

func (sql *sqlDBBackend) GetKey(keyId string, keyType string) (*c.Key, error) {
	k := sqlDBKey{}
	if sql.db.First(&k, keyId).RecordNotFound() {
		return nil, nil
	}
	return k.toKey(), nil
}

func (sql *sqlDBBackend) CreateUser(email string, name string, password string, profile map[string]interface{}) (*c.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	bprofile, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}

	u := sqlDBUser{
		Email:    email,
		Name:     name,
		Password: hash,
		Profile:  bprofile,
	}
	if err := sql.db.Create(&u).Error; err != nil {
		return nil, err
	}
	return u.toUser(), nil
}

func ActivateUser(userID string) (*c.User, error)
func ResetPassword(userID string) (string, error)
func UpdateProfile(userID string, name string, profile map[string]interface{}) (*c.User, error)
func Authenticate(email string, password string) *c.User
func AddKey(userID string, key *c.Key) error
func DeleteKey(userID string, keyID string) error
func GetAllKeys(userID string)

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
