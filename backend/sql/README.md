# SQL Database Plugins

This package contains plugins that connect to SQL databases. `sql-query` can
retrieve keys using a simple SELECT query and is adaptable to pretty much
any database you have already running. It's especially suited for keeping
your legacy keys working while moving operations over to a new user/key
infrastructure (perhaps powered by apiplexy).

`sql-full` offers full user/key management that apiplexy's Portal API can
plug into. Basically: instant developer self-service portal, as long as you
have an SQL database running somewhere. Right now, the full SQL plugin
insists on certain table and column names, so it's probably best to run it
in its own database/schema/tablespace.

## Common configuration options

Both plugins take the following options:

* `driver`: the name of the SQL database driver. Right now, the plugins
  support `sqlite3`, `postgres`, `mysql` and `mssql`.
* `connection_string`: a driver-specific connection string.

### Example connection strings

sqlite3:
```
/path/to/your/database.sqlite3
```

postgres:
```
host=localhost port=5432 user=your-user password=your-password dbname=apiplexy-db sslmode=enabled
```

mysql:
```
user:password@/dbname?charset=utf8&parseTime=True&loc=Local
```

mssql:
```
server=localhost;port=1433;user id=domain\user;password=your-password;database=apiplexy-db
```

## sql-query

This plugin performs a query to your backend database whenever an API key is
re-checked. You specify this query in SQL. The query **must** return exactly
four string values, namely the key **ID**, key **realm**, the name of the key's
**quota** and additional **data** attached to the key as a JSON string. The
columns must be returned in this exact order; their names are not checked. If
your database doesn't have values for one or more of these fields, SELECT
dummy values ('default' quota or empty JSON string).

In your query, you can use `:key_id` and `:key_type` as variables; they will
be replaced by apiplexy with the ID and type of the requested key.

An example:
```
SELECT id, key_realm, 'default' AS quota, '{}' AS json_data
FROM key_table
WHERE id = :key_id AND ktype = :key_type
```

`sql-query` takes the following additional configuration options:

* `query`: your query, as described above.

## sql-full

`sql-full` provides a full suite of user and key management functions. Just
put in your database connection information, and you can have a full self-
service portal up and running in seconds using apiplexy's portal API and the
built-in frontend.

`sql-full` takes the following additional configuration options:

* `create_tables`: Create user and key tables in your database if they don't
  already exist.


