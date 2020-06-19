// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/newrelic/infrastructure-agent/test/proxy/minagent"
	"github.com/newrelic/infrastructure-agent/test/proxy/testsetup"
	"github.com/sirupsen/logrus"
)

// minimalist agent management service. It exposes a `/restart` endpoint to kill the currently running agent and
// start a new agent process with the configuration received in the ConfigOptions payload
func main() {
	logrus.Info("Runing minimalistic test agent service...")
	runtime.GOMAXPROCS(1)

	lifecycles := make(chan *exec.Cmd, 1)
	lifecycles <- minagent.Start(minagent.ConfigOptions{})

	mux := http.NewServeMux()
	mux.HandleFunc("/restart", func(writer http.ResponseWriter, request *http.Request) {
		// kills previous agent
		(<-lifecycles).Process.Kill()

		// load configuration body
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			logrus.WithError(err).Error("Reading request body")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		options := minagent.ConfigOptions{}
		err = json.Unmarshal(body, &options)
		if err != nil {
			logrus.WithError(err).Error("parsing request body")
			writer.WriteHeader(http.StatusBadRequest)
		}

		// start new agent and return OK
		lifecycles <- minagent.Start(options)
		writer.WriteHeader(http.StatusOK)
	})

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", testsetup.AgentPort),
		Handler: mux,
	}

	logrus.Fatal(server.ListenAndServe())
}
