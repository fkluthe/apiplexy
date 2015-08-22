package backend

import (
	"encoding/json"
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
	UserID    string `sql:"not null;index"`
	CreatedAt time.Time
	DeletedAt *time.Time
}

func (k *sqlDBKey) toKey() *c.Key {
	ck := c.Key{
		ID:    k.ID,
		Realm: k.Realm,
		Type:  k.Type,
	}
	json.Unmarshal([]byte(k.Data), &ck.Data)
	return &ck
}

func (k *sqlDBKey) TableName() string {
	return "api_keys"
}

type sqlDBUser struct {
	ID        string
	Email     string `sql:"type:varchar(100);not null;unique_index"`
	Name      string
	Password  []byte `sql:"not null"`
	Admin     bool
	Active    bool
	Profile   string
	CreatedAt time.Time
	DeletedAt *time.Time
	LastLogin *time.Time
}

func (u *sqlDBUser) TableName() string {
	return "api_users"
}

func (u *sqlDBUser) toUser() *c.User {
	cu := c.User{
		ID:     u.ID,
		Email:  u.Email,
		Name:   u.Name,
		Admin:  u.Admin,
		Active: u.Active,
	}
	json.Unmarshal([]byte(u.Profile), &cu.Profile)
	return &cu
}

type SQLDBBackend struct {
	db *gorm.DB
}

func (sql *SQLDBBackend) GetKey(keyId string, keyType string) (*c.Key, error) {
	k := sqlDBKey{}
	if sql.db.First(&k, keyId).RecordNotFound() {
		return nil, nil
	}
	return k.toKey(), nil
}

func (sql *SQLDBBackend) CreateUser(email string, name string, password string, profile map[string]interface{}) (*c.User, error) {
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
		Profile:  string(bprofile[:]),
	}
	if err := sql.db.Create(&u).Error; err != nil {
		return nil, err
	}
	return u.toUser(), nil
}

func (sql *SQLDBBackend) ActivateUser(userID string) (*c.User, error) {
	u := sqlDBUser{}
	if sql.db.First(&u, userID).RecordNotFound() {
		return nil, fmt.Errorf("User not found.")
	}
	u.Active = true
	sql.db.Save(&u)
	return u.toUser(), nil
}

func (sql *SQLDBBackend) ResetPassword(userID string, newPassword string) error {
	u := sqlDBUser{}
	if sql.db.First(&u, userID).RecordNotFound() {
		return fmt.Errorf("User not found.")
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	u.Password = hash
	sql.db.Save(&u)
	return nil
}

func (sql *SQLDBBackend) UpdateProfile(userID string, name string, profile map[string]interface{}) (*c.User, error) {
	u := sqlDBUser{}
	if sql.db.First(&u, userID).RecordNotFound() {
		return nil, fmt.Errorf("User not found.")
	}
	u.Name = name
	bp, _ := json.Marshal(profile)
	u.Profile = string(bp[:])
	sql.db.Save(&u)
	return u.toUser(), nil
}

func (sql *SQLDBBackend) Authenticate(email string, password string) *c.User {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return nil
	}
	if bcrypt.CompareHashAndPassword(u.Password, []byte(password)) != nil {
		return nil
	}
	return u.toUser()
}

func (sql *SQLDBBackend) AddKey(userID string, keyType string, realm string, data map[string]interface{}) (*c.Key, error) {
	u := sqlDBUser{}
	if sql.db.First(&u, userID).RecordNotFound() {
		return nil, fmt.Errorf("User not found.")
	}
	bd, _ := json.Marshal(data)
	k := sqlDBKey{
		Realm: realm,
		Type:  keyType,
		Data:  string(bd[:]),
	}
	sql.db.Save(&k)
	return k.toKey(), nil
}

func (sql *SQLDBBackend) DeleteKey(userID string, keyID string) error {
	k := sqlDBKey{}
	if sql.db.First(&k, keyID).RecordNotFound() {
		return fmt.Errorf("Key does not exist.")
	}
	if k.UserID != userID {
		return fmt.Errorf("You are not the owner of this key.")
	}
	sql.db.Delete(&k)
	return nil
}

func (sql *SQLDBBackend) GetAllKeys(userID string) ([]*c.Key, error) {
	ks := []sqlDBKey{}
	if sql.db.Find(&ks, sqlDBKey{UserID: userID}).Error != nil {
		return nil, nil
	}
	cks := make([]*c.Key, len(ks))
	for i, k := range ks {
		cks[i] = k.toKey()
	}
	return cks, nil
}

// NewSQLDBBackend creates a backend plugin for popular SQL databases. It has the following
// configuration options (read from your config):
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
		return nil, fmt.Errorf("'%s' is not a valid driver for a SQL DB.", driverName)
	}

	return &SQLDBBackend{}, nil
}
