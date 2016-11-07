# Logrus PostgreSQL hook

## 1.0.1 - 2016-11-06

* AsyncHook now batches entries into DB, using a transaction every second (if there's something to log)
* InsertFunc is now private, and must be set at the hook level, not package

## 1.0.0 - 2016-11-03

* Initial version

