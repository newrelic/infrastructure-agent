// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// agent domain features
package log

import (
	"github.com/sirupsen/logrus"
)

// WithPlugin decorates log context with plugin name
func WithPlugin(name string) Entry {
	return func() *logrus.Entry {
		return w.l.WithField("plugin", name)
	}
}

// WithPlugin decorates entry context with plugin name
func (e Entry) WithPlugin(name string) Entry {
	return func() *logrus.Entry {
		return e().WithField("plugin", name)
	}
}

// WithIntegration decorates log context with integration name
func WithIntegration(name string) Entry {
	return func() *logrus.Entry {
		return w.l.WithField("integration", name)
	}
}

// WithIntegration decorates entry context with integration name
func (e Entry) WithIntegration(name string) Entry {
	return func() *logrus.Entry {
		return e().WithField("integration", name)
	}
}

// WithComponent decorates log context with integration name
func WithComponent(name string) Entry {
	return func() *logrus.Entry {
		return w.l.WithField("component", name)
	}
}

// WithComponent decorates entry context with integration name
func (e Entry) WithComponent(name string) Entry {
	return func() *logrus.Entry {
		return e().WithField("component", name)
	}
}
