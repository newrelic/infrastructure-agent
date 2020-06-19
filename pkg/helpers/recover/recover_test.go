// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package recover

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanicHandler(t *testing.T) {

	done := make(chan struct{})
	go func() {

		defer PanicHandler(LogAndFail)

		defer func(t *testing.T) {
			r := recover()

			assert.Empty(t, r)

			close(done)
		}(t)

		panic("")
	}()

	<-done
}
