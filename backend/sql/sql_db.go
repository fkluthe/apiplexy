package sql

import (
	"encoding/json"
	"fmt"
	"github.com/12foo/apiplexy"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type sqlDBKey struct {
	ID        string
	Realm     string
	Type      string
	Data      string
	UserID    string `sql:"not null;index"`
	CreatedAt time.Time
	DeletedAt *time.Time
}

func (k *sqlDBKey) toKey() *apiplexy.Key {
	ck := apiplexy.Key{
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

func (u *sqlDBUser) toUser() *apiplexy.User {
	cu := apiplexy.User{
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

func (sql *SQLDBBackend) GetKey(keyId string, keyType string) (*apiplexy.Key, error) {
	k := sqlDBKey{}
	if sql.db.First(&k, keyId).RecordNotFound() {
		return nil, nil
	}
	return k.toKey(), nil
}

func (sql *SQLDBBackend) CreateUser(email string, name string, password string, profile map[string]interface{}) (*apiplexy.User, error) {
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

func (sql *SQLDBBackend) ActivateUser(userID string) (*apiplexy.User, error) {
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

func (sql *SQLDBBackend) UpdateProfile(userID string, name string, profile map[string]interface{}) (*apiplexy.User, error) {
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

func (sql *SQLDBBackend) Authenticate(email string, password string) *apiplexy.User {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return nil
	}
	if bcrypt.CompareHashAndPassword(u.Password, []byte(password)) != nil {
		return nil
	}
	return u.toUser()
}

func (sql *SQLDBBackend) AddKey(userID string, keyType string, realm string, data map[string]interface{}) (*apiplexy.Key, error) {
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

func (sql *SQLDBBackend) GetAllKeys(userID string) ([]*apiplexy.Key, error) {
	ks := []sqlDBKey{}
	if sql.db.Find(&ks, sqlDBKey{UserID: userID}).Error != nil {
		return nil, nil
	}
	cks := make([]*apiplexy.Key, len(ks))
	for i, k := range ks {
		cks[i] = k.toKey()
	}
	return cks, nil
}

func (sql *SQLDBBackend) Name() string {
	return "sql-db"
}

func (sql *SQLDBBackend) Description() string {
	return "Use popular SQL databases as backend stores (full user/key management)."
}

func (sql *SQLDBBackend) DefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"driver":   "mysql/postgres/mssql/sqlite3",
		"host":     "localhost",
		"port":     5432,
		"database": "apiplexy-db",
		"user":     "apiplexy-user",
		"password": "my-secret-password",
		"ssl":      true,
	}
}

func (sql *SQLDBBackend) Configure(config map[string]interface{}) error {
	driverName := config["driver"]
	if driverName != "mysql" && driverName != "postgres" && driverName != "mssql" && driverName == "sqlite3" {
		return fmt.Errorf("'%s' is not a valid driver for a SQL DB.", driverName)
	}

	return nil
}

func init() {
	apiplexy.RegisterPlugin(&SQLDBBackend{})
}
