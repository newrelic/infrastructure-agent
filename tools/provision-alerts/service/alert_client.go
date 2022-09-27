// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package service

type AlertClient interface {
	Post(path string, body []byte) ([]byte, error)
	Put(path string, body []byte) ([]byte, error)
	Del(path string, body []byte) ([]byte, error)
	Get(path string, body []byte) ([]byte, error)
}
