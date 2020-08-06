// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"compress/gzip"
	context2 "context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	behttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	infra "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	remoteKey entity.Key = "remoteKey"
	ev                   = mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}
	evPost          = `[{"EntityID":13,"EntityKey":"agentKey","IsAgent":true,"Events":[{"entityKey":"agentKey","eventType":"TestEvent","value":"5"}],"ReportingAgentID":13}]`
	evPostRemote    = `[{"EntityID":1,"EntityKey":"remoteKey","IsAgent":false,"Events":[{"entityKey":"remoteKey","eventType":"TestEvent","value":"5"}],"ReportingAgentID":13}]`
	fixedProvideIDs = ProvideIDs(func(agentIdn entity.Identity, entities []identityapi.RegisterEntity) (ids []identityapi.RegisterEntityResponse, err error) {
		ids = []identityapi.RegisterEntityResponse{{ID: 1}}
		return
	})
)

const channelTimeout = 5 * time.Second

func knownRemoteKeyID() entity.KnownIDs {
	m := entity.NewKnownIDs()
	m.Put(remoteKey, entity.ID(1))

	return m
}

func TestMetricsEntryPointConfigVortex(t *testing.T) {
	ctx := &context{
		cfg: &config.Config{
			CollectorURL:          "http://test.com/",
			MetricsIngestEndpoint: "/metrics/",
		},
	}

	s := newVortexEventSender(ctx, "license", "userAgent", behttp.NullHttpClient, fixedProvideIDs, entity.NewKnownIDs())
	sender := s.(*vortexEventSender)

	assert.Equal(t, "http://test.com/metrics", sender.metricIngestURL)
}

func TestSingleEventBatchVortex(t *testing.T) {
	rc := infra.NewRequestRecorderClient()

	sender := newVortexEventSender(newContextWithVortex(), "license", "userAgent", rc.Client, fixedProvideIDs, entity.NewKnownIDs())
	assert.NoError(t, sender.Start())
	defer sender.Stop()

	assert.NoError(t, sender.QueueEvent(ev, ""))

	bodyRead, err := ioutil.ReadAll(waitFor(rc.RequestCh, channelTimeout).Body)
	assert.NoError(t, err)
	assert.Equal(t, evPost, string(bodyRead))
}

func TestSingleEventBatchCompressionVortex(t *testing.T) {
	rc := infra.NewRequestRecorderClient()

	c := newContextWithVortex()
	c.cfg.PayloadCompressionLevel = gzip.BestCompression

	sender := newVortexEventSender(c, "license", "userAgent", rc.Client, fixedProvideIDs, entity.NewKnownIDs())
	assert.NoError(t, sender.Start())
	defer sender.Stop()

	assert.NoError(t, sender.QueueEvent(ev, ""))

	req := waitFor(rc.RequestCh, channelTimeout)
	assert.Equal(t, req.Header.Get("Content-Encoding"), "gzip")
	gz, err := gzip.NewReader(req.Body)
	assert.NoError(t, err)
	bodyRead, err := ioutil.ReadAll(gz)
	assert.NoError(t, err)

	assert.Equal(t, evPost, string(bodyRead))
}

func TestLargeEventBatchVortex(t *testing.T) {
	rc := infra.NewRequestRecorderClient()

	sender := newVortexEventSender(newContextWithVortex(), "license", "userAgent", rc.Client, fixedProvideIDs, entity.NewKnownIDs())
	assert.NoError(t, sender.Start())
	defer sender.Stop()

	for i := 0; i < MAX_EVENT_BATCH_COUNT+10; i++ {
		assert.NoError(t, sender.QueueEvent(mapEvent{
			"eventType": "TestEvent",
			"value":     i,
		}, ""))
	}

	// We should have made two event batch posts since we went over the max batch size: MAX_EVENT_BATCH_COUNT + 10
	req1 := waitFor(rc.RequestCh, channelTimeout)
	req2 := waitFor(rc.RequestCh, channelTimeout)

	var postedBatches []MetricVortexPost
	bodyRead, err := ioutil.ReadAll(req1.Body)
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(bodyRead, &postedBatches))
	assert.Len(t, postedBatches, 1)
	assert.Len(t, postedBatches[0].Events, MAX_EVENT_BATCH_COUNT)

	bodyRead, err = ioutil.ReadAll(req2.Body)
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(bodyRead, &postedBatches))
	assert.Len(t, postedBatches, 1)
	assert.Len(t, postedBatches[0].Events, 10)

	assert.Equal(t, req1.Header.Get(behttp.LicenseHeader), "license")
	assert.Equal(t, req1.Header.Get(behttp.EntityKeyHeader), agentKey)
	assert.Equal(t, req1.Header.Get(behttp.AgentEntityIdHeader), agentIdn.ID.String())
	assert.Equal(t, req1.Header.Get("User-Agent"), "userAgent")
	assert.Equal(t, req1.Header.Get("Content-Type"), "application/json")
}

// Ensure the appropriate values for IsAgent and ExternalKeys are sent for remote entities
func TestBatchForRemoteEntityVortex(t *testing.T) {
	rc := infra.NewRequestRecorderClient()

	sender := newVortexEventSender(newContextWithVortex(), "license", "userAgent", rc.Client, fixedProvideIDs, knownRemoteKeyID())
	assert.NoError(t, sender.Start())
	defer sender.Stop()

	assert.NoError(t, sender.QueueEvent(ev, remoteKey))

	bodyRead, err := ioutil.ReadAll(waitFor(rc.RequestCh, channelTimeout).Body)
	assert.NoError(t, err)
	assert.Equal(t, evPostRemote, string(bodyRead))
}

func TestValidSSLCertVortex(t *testing.T) {
	sca, err := ioutil.TempFile("", "server-ca.pem")
	assert.NoError(t, err)
	scert, err := ioutil.TempFile("", "server-cert.pem")
	assert.NoError(t, err)
	skey, err := ioutil.TempFile("", "server-key.key")
	assert.NoError(t, err)
	sca.WriteString(serverCA)
	scert.WriteString(serverCert)
	skey.WriteString(serverKey)

	cert, err := tls.LoadX509KeyPair(scert.Name(), skey.Name())
	assert.NoError(t, err, "bad server certs")

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	ts.StartTLS()
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(u.Host)
	localhostURL := "https://localhost:" + port

	c := newContextWithVortex()
	c.cfg.CABundleFile = sca.Name()
	c.cfg.CollectorURL = localhostURL

	rc := infra.NewRequestRecorderClient()

	s := newVortexEventSender(c, "license", "userAgent", rc.Client, fixedProvideIDs, entity.NewKnownIDs())
	assert.NoError(t, s.Start())
	defer s.Stop()

	assert.NoError(t, s.QueueEvent(ev, ""))

	waitFor(rc.RequestCh, channelTimeout)
	time.Sleep(100 * time.Millisecond) // wait for err count to increase

	sender := s.(*vortexEventSender)
	count := atomic.LoadUint32(sender.sendErrorCount)
	assert.Equal(t, uint32(0), count, "The metric sender was unable to send the event")
}

func TestVortexEventSender_ResponseError(t *testing.T) {
	tests := map[string]struct {
		resp     http.Response
		backoffD time.Duration
	}{
		"Server error": {
			infra.ErrorResponse,
			1 * time.Second,
		},
		"Too many requests with retry after": {
			infra.TooManyRequestsResponse(infra.RetryAfter("42")),
			42 * time.Second,
		},
		"Too many requests with malformed retry after": {
			infra.TooManyRequestsResponse(infra.RetryAfter("MalformedHeader")),
			1 * time.Second,
		},
	}
	for tD, test := range tests {
		t.Run(tD, func(t *testing.T) {
			rc := infra.NewRequestRecorderClient(test.resp)
			s := newVortexEventSender(
				newContextWithVortex(),
				"license",
				"userAgent",
				rc.Client,
				fixedProvideIDs,
				entity.NewKnownIDs(),
			)
			sender := s.(*vortexEventSender)
			backoffCh := make(chan time.Duration)
			sender.getBackoffTimer = func(d time.Duration) *time.Timer {
				backoffCh <- d
				return time.NewTimer(0)
			}
			assert.NoError(t, sender.Start())
			defer sender.Stop()

			assert.NoError(t, sender.QueueEvent(ev, ""))

			waitFor(rc.RequestCh, channelTimeout)

			assert.Equal(t, test.backoffD, <-backoffCh)
			count := atomic.LoadUint32(sender.sendErrorCount)
			assert.Equal(t, uint32(1), count, "The metric sender was unable to send the event")
		})
	}
}

// Vortex specific

func TestVortexEventSender_QueueEvent_WaitsForAgentID(t *testing.T) {
	ctxNoID := newContextWithVortex()
	ctxNoID.SetAgentIdentity(entity.EmptyIdentity)

	rc := infra.NewRequestRecorderClient()
	sender := newVortexEventSender(ctxNoID, "license", "userAgent", rc.Client, fixedProvideIDs, knownRemoteKeyID())
	defer sender.Stop()

	go func() {
		require.NoError(t, sender.Start())
		require.NoError(t, sender.QueueEvent(ev, ""))
	}()

	// try to read if any request was sent
	req := waitFor(rc.RequestCh, 50*time.Millisecond)
	assert.Empty(t, req, "no events request should be sent until there is an agent-id set")

	ctxNoID.SetAgentIdentity(entity.Identity{ID: 123})

	req = waitFor(rc.RequestCh, channelTimeout)
	// allow metrics sender to retry, we might have to consider a backoff increase when retrying for this sender
	assert.NotEmpty(t, req)
}

func TestVortexEventSender_QueueEvent_DecoratesAgentID(t *testing.T) {
	rc := infra.NewRequestRecorderClient()
	sender := newVortexEventSender(newContextWithVortex(), "license", "userAgent", rc.Client, fixedProvideIDs, entity.NewKnownIDs())

	assert.NoError(t, sender.Start())
	defer sender.Stop()

	assert.NoError(t, sender.QueueEvent(ev, ""))

	bodyRead, err := ioutil.ReadAll(waitFor(rc.RequestCh, channelTimeout).Body)
	assert.NoError(t, err)
	assert.Equal(t, evPost, string(bodyRead))
}

func TestVortexEventSender_QueueEvent_DecoratesRemoteEntityID(t *testing.T) {
	rc := infra.NewRequestRecorderClient()
	sender := newVortexEventSender(newContextWithVortex(), "license", "userAgent", rc.Client, fixedProvideIDs, knownRemoteKeyID())

	assert.NoError(t, sender.Start())
	defer sender.Stop()

	assert.NoError(t, sender.QueueEvent(ev, remoteKey))

	bodyRead, err := ioutil.ReadAll(waitFor(rc.RequestCh, channelTimeout).Body)
	assert.NoError(t, err)
	assert.Equal(t, evPostRemote, string(bodyRead))
}

func newContextWithVortex() *context {
	var agentKeyVal atomic.Value
	agentKeyVal.Store(agentKey)
	c := &context{
		agentKey: agentKeyVal,
		cfg: &config.Config{
			ConnectEnabled:          true,
			PayloadCompressionLevel: gzip.NoCompression,
			RegisterConcurrency:     1,
			RegisterBatchSize:       1,
			RegisterFrequencySecs:   1,
		},
		id:           id.NewContext(context2.Background()),
		reconnecting: new(sync.Map),
	}
	c.SetAgentIdentity(agentIdn)

	return c
}

func waitFor(ch chan http.Request, timeout time.Duration) http.Request {
	var req http.Request
	select {
	case req = <-ch:
	case <-time.After(timeout):
	}
	return req
}
