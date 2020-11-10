// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package instrumentation

import (
	"net/http"
)

type Exporter interface {
	GetHandler() http.Handler
	IncrementSomething(val int64)
}
