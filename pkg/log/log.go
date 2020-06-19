// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package log provides a log wrapper to be used within the agent.
// It implements a Functional Logger Facade on top of Logrus. It aims at
// keeping conciseness without losing performance when composite loggers are built
// (WithError, WithFields...), making the related methods to be invoked lazily,
// avoiding to consume CPU resources if those are not going to be used (e.g. we won't
// generate a WithFields(...) logger entry if it is for debugging and the log level is Info.
package log

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// Entry is a functional wrapper for the logrus.Entry type
type Entry func() *logrus.Entry

func (e Entry) Debug(msg string) {
	if w.l.IsLevelEnabled(logrus.DebugLevel) {
		if w.smartVerboseEnabled() {
			w.cacheLog(e().Debug, msg)
		} else {
			e().Debug(msg)
		}
	}
}

func (e Entry) Debugf(format string, args ...interface{}) {
	if w.l.IsLevelEnabled(logrus.DebugLevel) {
		if w.smartVerboseEnabled() {
			w.cacheLog(e().Debug, fmt.Sprintf(format, args...))
		} else {
			e().Debugf(format, args...)
		}
	}
}

func (e Entry) Info(msg string) {
	if w.l.IsLevelEnabled(logrus.InfoLevel) {
		e().Info(msg)
	}
}

func (e Entry) Infof(format string, args ...interface{}) {
	if w.l.IsLevelEnabled(logrus.InfoLevel) {
		e().Infof(format, args...)
	}
}

func (e Entry) IsDebugEnabled() bool {
	return w.l.IsLevelEnabled(logrus.DebugLevel)
}

func (e Entry) IsWarnEnabled() bool {
	return w.l.IsLevelEnabled(logrus.WarnLevel)
}

func (e Entry) Warn(msg string) {
	if w.l.IsLevelEnabled(logrus.WarnLevel) {
		e().Warn(msg)
	}
}

func (e Entry) Error(msg string) {
	if w.l.IsLevelEnabled(logrus.ErrorLevel) {
		if w.smartVerboseEnabled() {
			w.mu.Lock()
			w.flush()
			w.mu.Unlock()
		}
		e().Error(msg)
	}
}

func (e Entry) WithFields(f logrus.Fields) Entry {
	return func() *logrus.Entry {
		return e().WithFields(f)
	}
}

func (e Entry) WithFieldsF(lff func() logrus.Fields) Entry {
	return func() *logrus.Entry {
		return e().WithFields(lff())
	}
}

func (e Entry) WithField(key string, value interface{}) Entry {
	return func() *logrus.Entry {
		return e().WithField(key, value)
	}
}

func (e Entry) WithError(err error) Entry {
	return func() *logrus.Entry {
		return e().WithError(err)
	}
}

func WithField(key string, value interface{}) Entry {
	return WithFieldsF(func() logrus.Fields {
		return logrus.Fields{key: value}
	})
}

func WithFields(f logrus.Fields) Entry {
	return func() *logrus.Entry {
		return w.l.WithFields(f)
	}
}

func WithFieldsF(lff func() logrus.Fields) Entry {
	return func() *logrus.Entry {
		return w.l.WithFields(lff())
	}
}

func WithError(err error) Entry {
	return func() *logrus.Entry {
		return w.l.WithError(err)
	}
}
