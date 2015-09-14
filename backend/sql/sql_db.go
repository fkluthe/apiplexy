package sql

import (
	gosql "database/sql"
	"encoding/json"
	"fmt"
	"github.com/12foo/apiplexy"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

type sqlDBKey struct {
	KeyID     string `sql:"primary_key" gorm:"primary_key"`
	Realm     string
	Type      string
	Data      string
	Quota     string
	User      string `sql:"not null;index"`
	CreatedAt time.Time
	DeletedAt *time.Time
}

func (k *sqlDBKey) toKey() *apiplexy.Key {
	ck := apiplexy.Key{
		ID:    k.KeyID,
		Realm: k.Realm,
		Type:  k.Type,
		Quota: k.Quota,
	}
	json.Unmarshal([]byte(k.Data), &ck.Data)
	return &ck
}

func (k *sqlDBKey) TableName() string {
	return "api_keys"
}

type sqlDBUser struct {
	Email     string `sql:"type:varchar(100);primary_key" gorm:"primary_key"`
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
		Email:  u.Email,
		Name:   u.Name,
		Admin:  u.Admin,
		Active: u.Active,
	}
	json.Unmarshal([]byte(u.Profile), &cu.Profile)
	return &cu
}

type SQLDBBackend struct {
	db gorm.DB
}

func (sql *SQLDBBackend) GetKey(keyId string, keyType string) (*apiplexy.Key, error) {
	k := sqlDBKey{}
	if sql.db.Where(sqlDBKey{KeyID: keyId, Type: keyType}).First(&k).RecordNotFound() {
		return nil, nil
	}
	return k.toKey(), nil
}

func (sql *SQLDBBackend) AddUser(email string, password string, user *apiplexy.User) (*apiplexy.User, error) {
	cu := sqlDBUser{}
	if !sql.db.Where(&sqlDBUser{Email: email}).First(&cu).RecordNotFound() {
		return nil, fmt.Errorf("A user with that email already exists.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if user.Profile == nil {
		user.Profile = make(map[string]interface{})
	}
	bprofile, err := json.Marshal(user.Profile)
	if err != nil {
		return nil, err
	}

	u := sqlDBUser{
		Email:    email,
		Name:     user.Name,
		Password: hash,
		Profile:  string(bprofile[:]),
	}
	if err := sql.db.Create(&u).Error; err != nil {
		return nil, err
	}
	return u.toUser(), nil
}

func (sql *SQLDBBackend) GetUser(email string) *apiplexy.User {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return nil
	}
	return u.toUser()
}

func (sql *SQLDBBackend) ActivateUser(email string) (*apiplexy.User, error) {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return nil, fmt.Errorf("User not found.")
	}
	sql.db.Model(&u).Where(&sqlDBUser{Email: email}).UpdateColumns(sqlDBUser{Active: true})
	return u.toUser(), nil
}

func (sql *SQLDBBackend) ResetPassword(email string, newPassword string) error {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return fmt.Errorf("User not found.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	sql.db.Model(&u).Where(&sqlDBUser{Email: email}).UpdateColumns(sqlDBUser{Password: hash})
	return nil
}

func (sql *SQLDBBackend) UpdateUser(email string, user *apiplexy.User) (*apiplexy.User, error) {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return nil, fmt.Errorf("User not found.")
	}
	bp, _ := json.Marshal(user.Profile)
	sql.db.Model(&u).Where(&sqlDBUser{Email: email}).UpdateColumns(sqlDBUser{Name: user.Name, Profile: string(bp[:])})
	return u.toUser(), nil
}

func (sql *SQLDBBackend) Authenticate(email string, password string) *apiplexy.User {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email, Active: true}).First(&u).RecordNotFound() {
		return nil
	}
	if bcrypt.CompareHashAndPassword(u.Password, []byte(password)) != nil {
		return nil
	}
	return u.toUser()
}

func (sql *SQLDBBackend) AddKey(email string, key *apiplexy.Key) (*apiplexy.Key, error) {
	u := sqlDBUser{}
	if sql.db.Where(&sqlDBUser{Email: email}).First(&u).RecordNotFound() {
		return nil, fmt.Errorf("User not found.")
	}
	bd, _ := json.Marshal(key.Data)
	k := sqlDBKey{
		KeyID: key.ID,
		Realm: key.Realm,
		Type:  key.Type,
		Quota: key.Quota,
		Data:  string(bd[:]),
		User:  email,
	}
	sql.db.Create(&k)
	return k.toKey(), nil
}

func (sql *SQLDBBackend) DeleteKey(email string, keyID string) error {
	k := sqlDBKey{}
	if sql.db.Where(sqlDBKey{KeyID: keyID}).First(&k).RecordNotFound() {
		return fmt.Errorf("Key does not exist.")
	}
	if k.User != email {
		return fmt.Errorf("You are not the owner of this key.")
	}
	sql.db.Delete(&k)
	return nil
}

func (sql *SQLDBBackend) GetAllKeys(email string) ([]*apiplexy.Key, error) {
	ks := []sqlDBKey{}
	if sql.db.Find(&ks, sqlDBKey{User: email}).Error != nil {
		return nil, nil
	}
	cks := make([]*apiplexy.Key, len(ks))
	for i, k := range ks {
		cks[i] = k.toKey()
	}
	return cks, nil
}

func (sql *SQLDBBackend) DefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"driver":        strings.Join(gosql.Drivers(), "/"),
		"host":          "localhost",
		"port":          5432,
		"database":      "apiplexy-db",
		"user":          "apiplexy-user",
		"password":      "my-secret-password",
		"ssl":           true,
		"create_tables": false,
	}
}

func (sql *SQLDBBackend) Configure(config map[string]interface{}) error {
	driverName := config["driver"]
	if driverName != "mysql" && driverName != "postgres" && driverName != "mssql" && driverName != "sqlite3" {
		return fmt.Errorf("'%s' is not a valid driver for a SQL DB.", driverName)
	}

	var err error
	var db gorm.DB

	switch driverName {
	case "sqlite3":
		db, err = gorm.Open("sqlite3", config["database"])
	case "postgres":
		var ssl string
		if config["ssl"].(bool) {
			ssl = "enabled"
		} else {
			ssl = "disabled"
		}
		db, err = gorm.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			config["host"], config["port"], config["user"], config["password"], config["database"], ssl))
	case "mysql":
		db, err = gorm.Open("mysql", fmt.Sprintf("%s:%s@%s:%d/%s?charset=utf8&parseTime=True&loc=Local",
			config["user"], config["password"], config["host"], config["port"], config["database"]))
	case "mssql":
		db, err = gorm.Open("postgres", fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;database=%s;encrypt=%b",
			config["host"], config["port"], config["user"], config["password"], config["database"], config["ssl"]))
	default:
		return fmt.Errorf("Unknown database driver: %s", driverName)
	}

	if err != nil {
		return fmt.Errorf("Error connecting to database. %s", err.Error())
	}

	if config["create_tables"].(bool) {
		if err = db.CreateTable(&sqlDBUser{}).Error; err != nil {
			return fmt.Errorf("Error creating user table: %s", err.Error())
		}
		if err = db.CreateTable(&sqlDBKey{}).Error; err != nil {
			return fmt.Errorf("Error creating key table: %s", err.Error())
		}
	}

	sql.db = db

	return nil
}

func init() {
	apiplexy.RegisterPlugin(
		"sql-db",
		"Use popular SQL databases as backend stores (full user/key management).",
		"https://github.com/12foo/apiplexy/tree/master/backend/sql",
		apiplexy.ManagementBackendPlugin(&SQLDBBackend{}),
	)
}
