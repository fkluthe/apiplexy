package sql

import (
	gosql "database/sql"
	"encoding/json"
	"fmt"
	"github.com/12foo/apiplexy"
	"regexp"
	"strings"
)

type SQLKeyBackend struct {
	query  string
	stmt   *gosql.Stmt
	argmap []bool
	db     *gosql.DB
}

func (sql *SQLKeyBackend) GetKey(keyId string, keyType string) (*apiplexy.Key, error) {
	args := make([]interface{}, len(sql.argmap))
	for i, isId := range sql.argmap {
		if isId {
			args[i] = keyId
		} else {
			args[i] = keyType
		}
	}
	row := sql.stmt.QueryRow(args...)
	k := apiplexy.Key{
		Type: keyType,
	}
	var jsonData string
	err := row.Scan(&k.ID, &k.Realm, &k.Quota, &jsonData)
	if err != nil {
		if err == gosql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	}
	if err = json.Unmarshal([]byte(jsonData), &k.Data); err != nil {
		return nil, err
	}
	return &k, nil
}

func (sql *SQLKeyBackend) DefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"driver":            strings.Join(gosql.Drivers(), "/"),
		"connection_string": "host=localhost port=5432 user=apiplexy password=apiplexy dbname=apiplexy",
		"query":             "SELECT key_id, realm, quota_name, json_data FROM table WHERE id = :key_id AND type = :key_type",
	}
}

func (sql *SQLKeyBackend) Configure(config map[string]interface{}) error {
	db, err := gosql.Open(config["driver"].(string), config["connection_string"].(string))
	if err != nil {
		return fmt.Errorf("Error connecting to database. %s", err.Error())
	}

	sql.db = db
	sql.query = config["query"].(string)

	replacer := regexp.MustCompile("(:key_id|:key_type)")
	argmap := make([]bool, 0)
	stmt := replacer.ReplaceAllFunc([]byte(sql.query), func(found []byte) []byte {
		f := string(found)
		if f == ":key_id" {
			argmap = append(argmap, true)
		} else if f == ":key_type" {
			argmap = append(argmap, false)
		}
		return []byte("?")
	})
	sql.stmt, err = db.Prepare(string(stmt))
	if err != nil {
		return fmt.Errorf("Error preparing SQL statement: %s", err.Error())
	}
	sql.argmap = argmap

	return nil
}

func init() {
	apiplexy.RegisterPlugin(
		"sql-query",
		"Perform simple key checks via query against a backend SQL database.",
		"https://github.com/12foo/apiplexy/tree/master/backend/sql",
		apiplexy.BackendPlugin(&SQLKeyBackend{}),
	)
}
