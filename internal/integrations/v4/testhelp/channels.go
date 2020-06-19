// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testhelp

import (
	"errors"
	"time"
)

// some helper functions to avoid tests boilerplate
const channelTimeout = 20 * time.Second

func ChannelRead(ch <-chan []byte) string {
	select {
	case str := <-ch:
		return string(str)
	case <-time.After(channelTimeout):
		return "timeout while waiting for an output line"
	}
}

var ErrChannelTimeout = errors.New("channel not closed after timeout")

// if the channel has been closed without an error, it returns nil. An error otherwise
func ChannelErrClosed(ch <-chan error) error {
	select {
	case err := <-ch:
		return err
	case <-time.After(channelTimeout):
		return ErrChannelTimeout
	}
}

func ChannelErrClosedTimeout(ch <-chan error, timeout time.Duration) error {
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return ErrChannelTimeout
	}
}
