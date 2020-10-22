// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Testing utils

package cmdchanneltest

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
)

func SuccessClient(serializedCmds string) commandapi.Client {
	httpClient := func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(serializedCmds))),
		}, nil
	}

	return commandapi.NewClient("https://foo", "123", "Agent v0", httpClient)
}
