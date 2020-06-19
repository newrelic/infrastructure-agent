// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package log

import (
	"bytes"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/sirupsen/logrus"
)

func TestSmartVerboseCachesDebugLogs(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)
	EnableSmartVerboseMode(1000)

	log := WithComponent("LogTester")
	log.Debug("I'm a test message :D")
	log.Debug("I'm another test message.")

	assert.Contains(t, w.logCache[0].msg, "I'm a test message :D")
	assert.Contains(t, w.logCache[1].msg, "I'm another test message")
}

func TestSmartVerboseLogsAfterErrorLog(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)
	EnableSmartVerboseMode(1000)

	log := WithComponent("LogTester")
	log.Debug("I'm a test message :D")
	log.Error("I'm an error :(")

	written := output.String()
	assert.Contains(t, written, "I'm a test message :D")
	assert.Contains(t, written, "I'm an error :(")
}

func TestSmartVerboseModeRingBuffer(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)
	EnableSmartVerboseMode(2)

	log := WithComponent("LogTester")
	log.Debug("Message 1")
	assert.Contains(t, w.logCache[0].msg, "Message 1")

	log.Debug("Message 2")
	assert.Contains(t, w.logCache[0].msg, "Message 1")
	assert.Contains(t, w.logCache[1].msg, "Message 2")

	log.Debug("Message 3")
	// The first log entry should now contain the second message
	assert.Contains(t, w.logCache[0].msg, "Message 2")
	assert.Contains(t, w.logCache[1].msg, "Message 3")

	log.Error("Error message")
	// Check Message 1 was dropped as limit was reached
	written := output.String()
	assert.NotContains(t, written, "Message 1")
	assert.Contains(t, written, "Message 2")
	assert.Contains(t, written, "Message 3")
	assert.Contains(t, written, "Error message")
}

func TestWithFields(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)
	log := WithFields(logrus.Fields{"abcdefg": "hijklm"})
	log.Warn("hello you")

	written := output.String()
	assert.Contains(t, written, "hello you")
	assert.Contains(t, written, "abcdefg")
	assert.Contains(t, written, "hijklm")
}

func TestWithFieldsChaining(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)
	log := WithField("123456", "78910").
		WithFields(logrus.Fields{"component": "supersampler"}).
		WithFieldsF(func() logrus.Fields {
			return logrus.Fields{"cool": "stuff"}
		})
	log.Warn("hello dude")

	written := output.String()
	assert.Contains(t, written, "hello dude")
	assert.Contains(t, written, "123456")
	assert.Contains(t, written, "78910")
	assert.Contains(t, written, "component")
	assert.Contains(t, written, "supersampler")
	assert.Contains(t, written, "cool")
	assert.Contains(t, written, "stuff")
}

func TestWithError(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)
	log := WithError(errors.New("catapun")).
		WithFields(logrus.Fields{"abcdefg": "hijklm"})
	log.Warn("something bad happened")

	written := output.String()
	assert.Contains(t, written, "something bad happened")
	assert.Contains(t, written, "abcdefg")
	assert.Contains(t, written, "hijklm")
	assert.Contains(t, written, "catapun")
}

func BenchmarkLogrusConditional(b *testing.B) {
	// disable and restore debug level
	level := logrus.GetLevel()
	defer func() {
		logrus.SetLevel(level)
	}()

	for _, level := range []logrus.Level{logrus.ErrorLevel, logrus.DebugLevel} {
		b.Run(level.String(), func(b *testing.B) {
			logrus.SetLevel(level)
			for i := 0; i < b.N; i++ {
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"some": "fields",
						"are":  "generated",
						"here": struct{}{},
					}).Debug("This message won't be displayed.")
				}
			}
		})
	}
}

func BenchmarkLogrusWithFields(b *testing.B) {
	// disable and restore debug level
	level := logrus.GetLevel()
	defer func() {
		logrus.SetLevel(level)
	}()

	for _, level := range []logrus.Level{logrus.ErrorLevel, logrus.DebugLevel} {
		b.Run(level.String(), func(b *testing.B) {
			logrus.SetLevel(level)

			for i := 0; i < b.N; i++ {
				logrus.WithFields(logrus.Fields{
					"some": "fields",
					"are":  "generated",
					"here": struct{}{},
				}).Debug("This message won't be displayed.")
			}
		})
	}
}

func BenchmarkFacade(b *testing.B) {
	// disable and restore debug level
	level := logrus.GetLevel()
	defer func() {
		logrus.SetLevel(level)
	}()

	for _, level := range []logrus.Level{logrus.ErrorLevel, logrus.DebugLevel} {
		b.Run(level.String(), func(b *testing.B) {
			logrus.SetLevel(level)
			for i := 0; i < b.N; i++ {
				WithFields(logrus.Fields{
					"some": "fields",
					"are":  "generated",
					"here": struct{}{},
				}).Debug("This message won't be displayed.")
			}
		})
	}
}

func BenchmarkFacadeFunc(b *testing.B) {
	// disable and restore debug level
	level := logrus.GetLevel()
	defer func() {
		logrus.SetLevel(level)
	}()
	for _, level := range []logrus.Level{logrus.ErrorLevel, logrus.DebugLevel} {
		b.Run(level.String(), func(b *testing.B) {
			logrus.SetLevel(level)

			for i := 0; i < b.N; i++ {
				WithFieldsF(func() logrus.Fields {
					return logrus.Fields{
						"some": "fields",
						"are":  "generated",
						"here": struct{}{},
					}
				}).Debug("This message won't be displayed.")
			}
		})
	}
}
