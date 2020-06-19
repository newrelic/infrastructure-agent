// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testhelpers

import (
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

type fakeHostnameResolver struct {
	full  string
	short string
	err   error
}

func (r *fakeHostnameResolver) Query() (full, short string, err error) {
	return r.full, r.short, r.err
}

func (r *fakeHostnameResolver) Long() string {
	return r.full
}

func (r *fakeHostnameResolver) AddObserver(_ string, _ chan<- hostname.ChangeNotification) {
	return
}

func (r *fakeHostnameResolver) RemoveObserver(_ string) {
	return
}

var NullHostnameResolver = &fakeHostnameResolver{}

func NewFakeHostnameResolver(full, short string, err error) hostname.ResolverChangeNotifier {
	return &fakeHostnameResolver{
		full:  full,
		short: short,
		err:   err,
	}
}
