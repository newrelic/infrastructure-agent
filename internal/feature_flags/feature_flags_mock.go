// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// nolint
package feature_flags

import "github.com/stretchr/testify/mock"

type FeatureFlagRetrieverMock struct {
	mock.Mock
}

func (f *FeatureFlagRetrieverMock) GetFeatureFlag(name string) (enabled, exists bool) {
	args := f.Called(name)

	return args.Bool(0), args.Bool(1)
}

func (f *FeatureFlagRetrieverMock) ShouldGetExistingFeatureFlag(name string, enabled bool) {
	f.
		On("GetFeatureFlag", name).
		Once().
		Return(enabled, true)
}

func (f *FeatureFlagRetrieverMock) ShouldGetFeatureFlag(name string, enabled bool, exists bool) {
	f.
		On("GetFeatureFlag", name).
		Once().
		Return(enabled, exists)
}

func (f *FeatureFlagRetrieverMock) ShouldNotGetFeatureFlag(name string) {
	f.
		On("GetFeatureFlag", name).
		Once().
		Return(false, false)
}

type FeatureFlagSetterMock struct {
	mock.Mock
}

func (f *FeatureFlagSetterMock) SetFeatureFlag(name string, enabled bool) error {
	args := f.Called(name, enabled)

	return args.Error(0)
}

func (f *FeatureFlagSetterMock) ShouldReturnNoError(name string) {
	f.
		On("SetFeatureFlag", name, mock.Anything).
		Once().
		Return(nil)
}

func (f *FeatureFlagSetterMock) ShouldReturnError(name string, err error) {
	f.
		On("SetFeatureFlag", name, mock.Anything).
		Once().
		Return(err)
}
