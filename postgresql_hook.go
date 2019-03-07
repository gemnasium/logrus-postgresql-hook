package pglogrus

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Set pglogrus.BufSize = <value> _before_ calling NewHook
// Once the buffer is full, logging will start blocking, waiting for slots to
// be available in the queue.
var BufSize uint = 8192

// Hook to send logs to a PostgreSQL database
type Hook struct {
	Extra      map[string]interface{}
	db         *sql.DB
	mu         sync.RWMutex
	InsertFunc func(*sql.DB, *logrus.Entry) error
	filters    []filter
}

type AsyncHook struct {
	*Hook
	buf        chan *logrus.Entry
	flush      chan bool
	wg         sync.WaitGroup
	Ticker     *time.Ticker
	InsertFunc func(*sql.Tx, *logrus.Entry) error
}

var insertFunc = func(db *sql.DB, entry *logrus.Entry) error {
	jsonData, err := json.Marshal(entry.Data)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO logs(level, message, message_data, created_at) VALUES ($1,$2,$3,$4);", entry.Level, entry.Message, jsonData, entry.Time)
	return err
}

var asyncInsertFunc = func(txn *sql.Tx, entry *logrus.Entry) error {
	jsonData, err := json.Marshal(entry.Data)
	if err != nil {
		return err
	}

	_, err = txn.Exec("INSERT INTO logs(level, message, message_data, created_at) VALUES ($1,$2,$3,$4);", entry.Level, entry.Message, jsonData, entry.Time)
	return err
}

type filter func(*logrus.Entry) *logrus.Entry

// NewHook creates a PGHook to be added to an instance of logger.
func NewHook(db *sql.DB, extra map[string]interface{}) *Hook {
	return &Hook{
		Extra:      extra,
		db:         db,
		InsertFunc: insertFunc,
		filters:    []filter{},
	}
}

// NewAsyncHook creates a hook to be added to an instance of logger.
// The hook created will be asynchronous, and it's the responsibility of the user to call the Flush method
// before exiting to empty the log queue.
func NewAsyncHook(db *sql.DB, extra map[string]interface{}) *AsyncHook {
	hook := &AsyncHook{
		Hook:       NewHook(db, extra),
		buf:        make(chan *logrus.Entry, BufSize),
		flush:      make(chan bool),
		Ticker:     time.NewTicker(300 * time.Millisecond),
		InsertFunc: asyncInsertFunc,
	}
	go hook.fire() // Log in background
	return hook
}

func (hook *Hook) Fire(entry *logrus.Entry) error {
	newEntry := hook.newEntry(entry)
	if newEntry == nil {
		// entry is ignored.
		return nil
	}
	return hook.InsertFunc(hook.db, newEntry)

}

// Fire is called when a log event is fired.
// We assume the entry will be altered by another hook,
// otherwise we might logging something wrong to PostgreSQL
func (hook *AsyncHook) Fire(entry *logrus.Entry) error {
	newEntry := hook.newEntry(entry)
	if newEntry == nil {
		// entry is ignored.
		return nil
	}
	hook.wg.Add(1)
	hook.buf <- newEntry
	return nil
}

// newEntry will prepare a new logrus entry to be logged in the DB
// the extra fields are added to entry Data
func (hook *Hook) newEntry(entry *logrus.Entry) *logrus.Entry {
	hook.mu.RLock() // Claim the mutex as a RLock - allowing multiple go routines to log simultaneously
	defer hook.mu.RUnlock()

	// Don't modify entry.Data directly, as the entry will used after this hook was fired
	data := map[string]interface{}{}

	// Merge extra fields
	for k, v := range hook.Extra {
		data[k] = v
	}
	for k, v := range entry.Data {
		data[k] = v
		if k == logrus.ErrorKey {
			asError, isError := v.(error)
			_, isMarshaler := v.(json.Marshaler)
			if isError && !isMarshaler {
				data[k] = newMarshalableError(asError)
			}
		}
	}

	newEntry := &logrus.Entry{
		Logger:  entry.Logger,
		Data:    data,
		Time:    entry.Time,
		Level:   entry.Level,
		Caller:  entry.Caller,
		Message: entry.Message,
	}

	// Apply filters
	for _, fn := range hook.filters {
		newEntry = fn(newEntry)
		if newEntry == nil {
			break
		}
	}
	return newEntry
}

// Levels returns the available logging levels.
func (hook *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

// Blacklist filters entry field values.
// This useful when you want your application to log extra fields locally
// but don't want pg to store them.
func (hook *Hook) Blacklist(b []string) {
	hook.AddFilter(blackListFilter(b))
}

// Flush waits for the log queue to be empty.
// This func is meant to be used when the hook was created with NewAsyncHook.
func (hook *AsyncHook) Flush() {
	hook.wg.Wait()
	hook.flush <- true
	<-hook.flush
}

// fire will loop on the 'buf' channel, and write entries to pg
func (hook *AsyncHook) fire() {
	for {
		var err error
		txn, err := hook.db.Begin()
		if err != nil {
			fmt.Fprintln(os.Stderr, "[pglogrus] Can't create db transaction:", err)
			// Don't create new transactions too fast, it will flood stderr
			select {
			case <-hook.Ticker.C:
				continue
			}
		}

		var numEntries int
	Loop:
		for {
			select {
			case entry := <-hook.buf:
				err = hook.InsertFunc(txn, entry)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[pglogrus] Can't insert entry (%v): %v\n", entry, err)
				}
				numEntries++
			case <-hook.Ticker.C:
				if numEntries > 0 {
					break Loop
				}
			case <-hook.flush:
				err = txn.Commit()
				if err != nil {
					fmt.Fprintln(os.Stderr, "[pglogrus] Can't commit transaction:", err)
				}
				hook.flush <- true
				return
			}

		}

		err = txn.Commit()
		if err != nil {
			fmt.Fprintln(os.Stderr, "[pglogrus] Can't commit transaction:", err)
		}

		for i := 0; i < numEntries; i++ {
			hook.wg.Done()
		}
	}
}

func (hook *Hook) Close() error {
	return hook.db.Close()
}

//AddFilter adds filter that can modify or ignore entry.
func (hook *Hook) AddFilter(fn filter) {
	hook.filters = append(hook.filters, fn)
}

func blackListFilter(blacklist []string) filter {
	return func(entry *logrus.Entry) *logrus.Entry {
		for _, name := range blacklist {
			delete(entry.Data, name)
		}
		return entry
	}
}
