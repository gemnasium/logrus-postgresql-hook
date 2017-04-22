# Logrus PostgreSQL hook

## 1.0.3 - 2017-04-24

* Fix log flushing. The previous version was leaving a DB transaction open on exit.

## 1.0.2 - 2016-11-08

* Add hook.Close() func to close the DB from the hook

## 1.0.1 - 2016-11-06

* AsyncHook now batches entries into DB, using a transaction every second (if there's something to log)
* InsertFunc is now private, and must be set at the hook level, not package

## 1.0.0 - 2016-11-03

* Initial version

