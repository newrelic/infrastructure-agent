/*
 * Copyright 2021 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package fflag

import "github.com/stretchr/testify/mock"

type FBRestarterMock struct {
	mock.Mock
}

func (f *FBRestarterMock) Restart() error {
	fArgs := f.Called()

	return fArgs.Error(0) //nolint:wrapcheck
}

func (f *FBRestarterMock) ShouldReturnNoError() {
	f.
		On("Restart").
		Once().
		Return(nil)
}

func (f *FBRestarterMock) ShouldReturnError(err error) {
	f.
		On("Restart").
		Once().
		Return(err)
}
