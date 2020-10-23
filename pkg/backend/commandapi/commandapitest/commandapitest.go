// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package commandapitest

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
)

const serializedCmds = `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "flag1",
					"enabled": true
				}
			},
			{
				"id": 0,
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 3000
				}
			}
		]
	}
`

type HttpClient struct {
	returnStatus    int
	body            string
	returnErr       error
	ReceivedPayload string
}

func ClientReturns(status int, body string, err error) *HttpClient {
	return &HttpClient{
		returnStatus: status,
		body:         body,
		returnErr:    err,
	}
}

func (c *HttpClient) Do(req *http.Request) (res *http.Response, err error) {
	if req.Method == http.MethodPost {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		c.ReceivedPayload = string(b)
	}
	return &http.Response{
		Status:     "foo",
		StatusCode: c.returnStatus,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(c.body))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, c.returnErr
}

func TrimJSON(json string) string {
	return strings.Replace(strings.Replace(strings.Replace(json, "\n", "", -1), " ", "", -1), "\t", "", -1)
}
