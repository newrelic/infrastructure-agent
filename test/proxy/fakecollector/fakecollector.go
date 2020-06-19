// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fakecollector

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"

	"github.com/sirupsen/logrus"
)

// Recorded request, storing the samples that were contained in the request
type Request struct {
	Samples []map[string]interface{}
}

type FakeCollector struct {
	requests        chan Request
	notifiedProxies map[string]interface{}
}

func NewService() FakeCollector {
	return FakeCollector{
		requests:        make(chan Request, 1000),
		notifiedProxies: map[string]interface{}{},
	}
}

func (fc *FakeCollector) Connect(writer http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		logrus.WithError(err).Error("Reading request body")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := struct {
		Fingerprint fingerprint.Fingerprint `json:"fingerprint"`
		Type        string                  `json:"type"`
		Protocol    string                  `json:"protocol"`
		EntityID    entity.ID               `json:"entityId,omitempty"`
	}{}

	if err := json.Unmarshal(body, &req); err != nil {
		logrus.WithError(err).Error("parsing request body map")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if request.Method == http.MethodPost {
		logrus.WithField("connect", req).Info("received connect fingerprint")
	} else if request.Method == http.MethodPut && req.EntityID > 0 {
		logrus.WithField("reconnect", req).Info("received update fingerprint")
	} else {
		writer.WriteHeader(http.StatusBadRequest)
	}

	writer.WriteHeader(http.StatusOK)
}

func (fc *FakeCollector) NewSample(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		logrus.WithError(err).Error("Reading request body")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := Request{
		Samples: []map[string]interface{}{},
	}

	if err := json.Unmarshal(body, &req.Samples); err != nil {
		logrus.WithError(err).Error("parsing request body map")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	logrus.WithField("request", req).Info("received payload")

	fc.requests <- req
	writer.WriteHeader(http.StatusOK)
}

func (fc *FakeCollector) DequeueSample(writer http.ResponseWriter, request *http.Request) {
	body, err := json.Marshal(<-fc.requests)
	if err != nil {
		logrus.WithError(err).Error("unmarshaling dequeued request")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.Write(body)
}

func (fc *FakeCollector) ClearQueue(writer http.ResponseWriter, request *http.Request) {
	fc.requests = make(chan Request, 1000)
	fc.notifiedProxies = map[string]interface{}{}
	writer.WriteHeader(http.StatusOK)
}

func (fc *FakeCollector) NotifyProxy(resp http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		proxy, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logrus.WithError(err).Error("reading proxy notification")
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		fc.notifiedProxies[string(proxy)] = 1
		resp.WriteHeader(http.StatusAccepted)
	}
}

func (fc *FakeCollector) LastProxy(resp http.ResponseWriter, req *http.Request) {
	proxies := make([]string, 0)
	for k := range fc.notifiedProxies {
		proxies = append(proxies, k)
	}
	_, err := resp.Write([]byte(strings.Join(proxies, ",")))
	if err != nil {
		logrus.WithError(err).Error("getting latest proxies")
	}
	resp.WriteHeader(http.StatusOK)
}
