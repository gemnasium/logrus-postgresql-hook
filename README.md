# PostgreSQL Hook for [Logrus](https://github.com/Sirupsen/logrus) <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:" />&nbsp;[![Build Status](https://travis-ci.org/gemnasium/logrus-postgresql-hook.svg?branch=master)](https://travis-ci.org/gemnasium/logrus-postgresql-hook)&nbsp;[![godoc reference](https://godoc.org/github.com/gemnasium/logrus-postgresql-hook?status.svg)](https://godoc.org/github.com/gemnasium/logrus-postgresql-hook)

Use this hook to send your logs to [postgresql](http://postgresql.org) server.

## Usage

The hook must be configured with:

* A postgresql db connection (*`*sql.DB`)
* an optional hash with extra global fields. These fields will be included in all messages sent to postgresql

```go
package main

import (
    log "github.com/Sirupsen/logrus"
    "gopkg.in/gemnasium/logrus-postgresql-hook.v1"
    )

func main() {
    db, err := sql.Open("postgres", "user=postgres dbname=postgres host=postgres sslmode=disable")
      if err != nil {
        t.Fatal("Can't connect to postgresql database:", err)
      }
    defer db.Close()
    hook := pglorus.NewHook(db, map[string]interface{}{"this": "is logged every time"})
    log.Hooks.Add(hook)
    log.Info("some logging message")
}
```

### Asynchronous logger

This package provides an asynchronous hook, so logging won't block waiting for the data to be inserted in the DB.
Be careful to defer call `hook.Flush()` if you are using this kind of hook.


```go
package main

import (
    log "github.com/Sirupsen/logrus"
    "gopkg.in/gemnasium/logrus-postgresql-hook.v1"
    )

func main() {
    db, err := sql.Open("postgres", "user=postgres dbname=postgres host=postgres sslmode=disable")
      if err != nil {
        t.Fatal("Can't connect to postgresql database:", err)
      }
    defer db.Close()
    hook := pglorus.NewAsyncHook(db, map[string]interface{}{"this": "is logged every time"})
    defer hook.Flush()
    log.Hooks.Add(hook)
    log.Info("some logging message")
}
```


### Customize insertion

By defaults, the hook will log into a `logs` table (cf the test schema in `migrations`).
To change this behavior, set the `InsertFunc` before creating the hook:

```go
package main

import (
    log "github.com/Sirupsen/logrus"
    "gopkg.in/gemnasium/logrus-postgresql-hook.v1"
    )

func main() {
    db, err := sql.Open("postgres", "user=postgres dbname=postgres host=postgres sslmode=disable")
      if err != nil {
        t.Fatal("Can't connect to postgresql database:", err)
      }
    defer db.Close()

    pglogrus.InsertFunc = func(db *sqlDB, entry *logrus.Entry) error {
      jsonData, err := json.Marshal(entry.Data)
        if err != nil {
          return err
        }

      _, err = db.Exec("INSERT INTO another_logs_table(level, message, message_data, created_at) VALUES ($1,$2,$3,$4);", entry.Level, entry.Message, jsonData, entry.Time)
        return err
    }
    hook := pglorus.NewHook(db, map[string]interface{}{"this": "is logged every time"})
    log.Hooks.Add(hook)
    log.Info("some logging message")
}
```


## Run tests

Since this hook is hitting a DB, we're testing again a real PostgreSQL server:

    docker-compose run --rm test
