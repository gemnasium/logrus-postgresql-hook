# Logrus PostgreSQL hook

## 1.1.3 - 2019-03-07

* Support for `logrus.TraceLevel`

## 1.1.2 - 2019-03-07

* Fix a race condition when using hook's ticker. *Breaking change*: hook.Ticker is not exported anymore. If you need to change to loop duration, please use `LoopDuration(time.Duration)` instead now. (#9)

## 1.1.1 - 2019-02-12

* Fix bug where `Caller` was not passed to following hooks (#6)

## 1.1.0 - 2017-07-24

* New `AddFilter` method for hooks. This method provides a better way to process the logged entry.
* Code cleaning. Blacklist([]string) has been kept to preserve retro-compatibility, but it's now using an internal filter.

## 1.0.4 - 2017-06-01

* Update import logrus path (see https://github.com/sirupsen/logrus/pull/384)

## 1.0.3 - 2017-04-24

* Fix log flushing. The previous version was leaving a DB transaction open on exit.

## 1.0.2 - 2016-11-08

* Add hook.Close() func to close the DB from the hook

## 1.0.1 - 2016-11-06

* AsyncHook now batches entries into DB, using a transaction every second (if there's something to log)
* InsertFunc is now private, and must be set at the hook level, not package

## 1.0.0 - 2016-11-03

* Initial version

