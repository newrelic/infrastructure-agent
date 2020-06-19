// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
)

var (
	backoffSecs int
	ffEnabled   bool
	ffName      string
	errCode     int
)

func init() {
	flag.IntVar(&backoffSecs, "backoff", 0, "set a backoff response (secs)")
	flag.BoolVar(&ffEnabled, "ff_enabled", false, "FF is enabled")
	flag.StringVar(&ffName, "ff_name", "docker_enabled", "FF name")
	flag.IntVar(&errCode, "err_code", 0, "return error code")
}

func main() {
	flag.Parse()

	handler := returnSuccess(ffName, ffEnabled)
	if backoffSecs != 0 {
		handler = returnBackoff(backoffSecs)
	}
	if errCode != 0 {
		handler = returnError(errCode)
	}

	http.HandleFunc("/agent_commands/v1/commands", handler)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic("cannot create web server:" + err.Error())
	}
}

func returnSuccess(ffName string, ffEnabled bool) http.HandlerFunc {
	tmpl := `{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "%s",
					"enabled": %s
				}
			}
		]
	}`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		payload := fmt.Sprintf(tmpl, ffName, strconv.FormatBool(ffEnabled))
		_, _ = w.Write([]byte(payload))
		fmt.Println(payload)

	}
}

func returnBackoff(backoffSecs int) http.HandlerFunc {
	tmpl := `{
		"return_value": [
			{
				"id": 0,
				"name": "backoff_command_channel",
				"arguments": {
					"delay": %s
				}
			}
		]
	}`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		payload := fmt.Sprintf(tmpl, strconv.Itoa(backoffSecs))
		_, _ = w.Write([]byte(payload))
		fmt.Println(payload)
	}
}

func returnError(statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}
}
