// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package log is our own log package, basically a wrapper for logrus with some improvements like:
// - lazy evaluation of WithFields funcs
// - shrinked logrus API (no Fatal, Panic...)
// - added agent domain API helpers like CustomIntegration/...
// this file was based on logrus/exported.go
package log

import (
	"io"
	"sync"

	"github.com/sirupsen/logrus"
)

// logrus wrapper
type wrap struct {
	l                *logrus.Logger
	smartVerboseMode bool

	// Stores log entries in Smart Verbose mode
	logCache           []log
	cachedEntryCounter int
	cachedEntryLimit   int
	mu                 *sync.Mutex
}

type log struct {
	f   func(...interface{})
	msg string
}

// usual singleton access used on the codebase
var w = wrap{
	l:  logrus.StandardLogger(),
	mu: &sync.Mutex{},
}

func (w *wrap) smartVerboseEnabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.smartVerboseMode
}

func (w *wrap) setLogCache(entryLimit int) {
	w.logCache = make([]log, 0, entryLimit)
	w.cachedEntryLimit = entryLimit
}

func (w *wrap) cacheLog(logF func(...interface{}), msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	cursor := 0
	if w.cachedEntryCounter >= w.cachedEntryLimit {
		cursor = 1
	}
	w.logCache = append(w.logCache[cursor:], log{f: logF, msg: msg})
	w.cachedEntryCounter++
}

func (w *wrap) flush() {
	for _, log := range w.logCache {
		log.f(log.msg)
	}
	w.setLogCache(w.cachedEntryLimit)
	w.cachedEntryCounter = 0
}

func EnableSmartVerboseMode(cachedEntryLimit int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.smartVerboseMode = true
	w.setLogCache(cachedEntryLimit)
	SetLevel(logrus.DebugLevel)
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	w.l.SetOutput(out)
}

// AddHook adds a hook to the singleton logger used in the codebase
func AddHook(hook logrus.Hook) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.l.Hooks.Add(hook)
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter logrus.Formatter) {
	w.l.SetFormatter(formatter)
}

// SetLevel sets the standard logger level.
func SetLevel(level logrus.Level) {
	w.l.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() logrus.Level {
	return w.l.GetLevel()
}

// IsLevelEnabled checks if the log level of the standard logger is greater than the level param
func IsLevelEnabled(level logrus.Level) bool {
	return w.l.IsLevelEnabled(level)
}

// WithError creates an entry from the standard logger and adds an error to it, using the value defined in ErrorKey as key.
//func WithError(err error) *logrus.Entry {
//	return w.l.WithField(logrus.ErrorKey, err)
//}

// WithField creates an entry from the standard logger and adds a field to
// it. If you want multiple fields, use `WithFields`.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
//func WithField(key string, value interface{}) *logrus.Entry {
//	return w.l.WithField(key, value)
//}

// WithFields creates an entry from the standard logger and adds multiple
// fields to it. This is simply a helper for `WithField`, invoking it
// once for each field.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
//func WithFields(fields logrus.Fields) *logrus.Entry {
//	return w.l.WithFields(fields)
//}

// WithTime creats an entry from the standard logger and overrides the time of
// logs generated with it.
//

// Trace logs a message at level Trace on the standard logger.
func Trace(args ...interface{}) {
	w.l.Trace(args...)
}

// Debug logs a message at level Debug on the standard logger.
func Debug(args ...interface{}) {
	w.l.Debug(args...)
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	w.l.Info(args...)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	w.l.Warn(args...)
}

// Warning logs a message at level Warn on the standard logger.
func Warning(args ...interface{}) {
	w.l.Warning(args...)
}

// Error logs a message at level Error on the standard logger.
func Error(args ...interface{}) {
	w.l.Error(args...)
}

// Tracef logs a message at level Trace on the standard logger.
func Tracef(format string, args ...interface{}) {
	w.l.Tracef(format, args...)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	w.l.Debugf(format, args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	w.l.Infof(format, args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	w.l.Warnf(format, args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	w.l.Errorf(format, args...)
}
