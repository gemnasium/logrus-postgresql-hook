package pglogrus

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/sirupsen/logrus"
)

func TestHooks(t *testing.T) {
	db, err := sql.Open("postgres", "user=postgres dbname=postgres host=postgres sslmode=disable")
	if err != nil {
		t.Fatal("Can't connect to postgresql test database:", err)
	}
	defer db.Close()

	hooks := map[string]interface {
		logrus.Hook
		Blacklist([]string)
		AddFilter(filter)
	}{
		"Hook":       NewHook(db, map[string]interface{}{}),
		"Async Hook": NewAsyncHook(db, map[string]interface{}{}),
	}

	for name, hook := range hooks {
		t.Run(name, func(t *testing.T) {
			hook.Blacklist([]string{"filterMe"})
			hook.AddFilter(func(entry *logrus.Entry) *logrus.Entry {
				if _, ok := entry.Data["ignore"]; ok {
					// ignore entry
					entry = nil
				}
				return entry
			})

			log := logrus.New()
			log.Out = ioutil.Discard
			log.Level = logrus.DebugLevel
			log.Hooks.Add(hook)

			if h, ok := hook.(*AsyncHook); ok {
				h.Ticker = time.NewTicker(100 * time.Millisecond)
			}

			// Purge our test DB
			_, err = db.Exec("delete from logs;")
			if err != nil {
				t.Fatal("Can't purge DB:", err)
			}

			msg := "test message\nsecond line"
			errMsg := "some error occurred"

			var wg sync.WaitGroup

			messages := []*logrus.Entry{
				{
					Logger:  log,
					Data:    logrus.Fields{"withField": "1", "user": "123"},
					Level:   logrus.ErrorLevel,
					Caller:  &runtime.Frame{Function: "somefunc"},
					Message: errMsg,
				},
				{
					Logger:  log,
					Data:    logrus.Fields{"withField": "2", "filterMe": "1"},
					Level:   logrus.InfoLevel,
					Caller:  &runtime.Frame{Function: "somefunc"},
					Message: msg,
				},
				{
					Logger:  log,
					Data:    logrus.Fields{"withField": "3"},
					Level:   logrus.DebugLevel,
					Caller:  &runtime.Frame{Function: "somefunc"},
					Message: msg,
				},
				{
					Logger:  log,
					Data:    logrus.Fields{"ignore": "me"},
					Level:   logrus.InfoLevel,
					Caller:  &runtime.Frame{Function: "somefunc"},
					Message: msg,
				},
			}

			for _, entry := range messages {
				wg.Add(1)
				go func(e *logrus.Entry) {
					defer wg.Done()
					switch e.Level {
					case logrus.DebugLevel:
						e.Debug(e.Message)
					case logrus.InfoLevel:
						e.Info(e.Message)
					case logrus.ErrorLevel:
						e.Error(e.Message)
					default:
						t.Error("unknown level:", e.Level)
					}
				}(entry)
			}
			wg.Wait()

			if h, ok := hook.(*AsyncHook); ok {
				h.Flush()
			}

			// Check results in DB
			var (
				data       *json.RawMessage
				created_at time.Time
				level      logrus.Level
				message    string
			)
			rows, err := db.Query("select level, message, message_data, created_at from logs")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var numRows int
			for rows.Next() {
				numRows++
				err := rows.Scan(&level, &message, &data, &created_at)
				if err != nil {
					t.Fatal(err)
				}

				var expectedData map[string]interface{}
				var expectedMsg string

				switch level {
				case logrus.ErrorLevel:
					expectedMsg = errMsg
					expectedData = map[string]interface{}{
						"withField": "1",
						"user":      "123",
					}

				case logrus.InfoLevel:
					expectedMsg = msg
					expectedData = map[string]interface{}{
						"withField": "2",
						// "filterme" should be filtered
					}
				case logrus.DebugLevel:
					expectedMsg = msg
					expectedData = map[string]interface{}{
						"withField": "3",
					}
				default:
					t.Error("Unknown log level:", level)
				}

				if message != expectedMsg {
					t.Errorf("Expected message to be %q, got %q\n", expectedMsg, message)
				}

				var storedData map[string]interface{}
				if err := json.Unmarshal(*data, &storedData); err != nil {
					t.Fatal("Can't unmarshal data from DB: ", err)
				}
				if !reflect.DeepEqual(expectedData, storedData) {
					t.Errorf("Expected stored data to be %v, got %v\n", expectedData, storedData)
				}

			}
			err = rows.Err()
			if err != nil {
				t.Fatal(err)
			}
			if len(messages)-1 != numRows {
				t.Errorf("Expected %d rows, got %d\n", len(messages), numRows)
			}

		})
	}
}
