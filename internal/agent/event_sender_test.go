// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	infra "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"sync"

	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	. "gopkg.in/check.v1"
)

type EventSenderSuite struct {
}

var _ = Suite(&EventSenderSuite{})

func (s *EventSenderSuite) TestMetricsEntryPointConfig(c *C) {
	context := newTestContext("testAgent",
		&config.Config{
			CollectorURL:          "http://test.com/",
			MetricsIngestEndpoint: "/metrics/",
		})
	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)

	c.Assert(sender.metricIngestURL, Equals, "http://test.com/metrics")
}

func (s *EventSenderSuite) TestSingleEventBatch(c *C) {
	accumulatedBatches := make(map[int][]byte) // A map of number -> event payload
	// We're using a map so we can add to it within the handler function and later access the data.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		c.Assert(err, IsNil)
		accumulatedBatches[len(accumulatedBatches)] = body
	}))
	defer ts.Close()

	context := newTestContext("testAgent",
		&config.Config{
			PayloadCompressionLevel: gzip.NoCompression,
			CollectorURL:            ts.URL,
		})

	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "")

	// Waiting 2x the batch ticker should give plenty of time for it to process
	time.Sleep(EVENT_BATCH_TIMER_DURATION * 2 * time.Second)

	c.Assert(string(accumulatedBatches[0]), Equals, `[{"ExternalKeys":["testAgent"],"IsAgent":true,"Events":[{"eventType":"TestEvent","value":"5"}]}]`)
}

func (s *EventSenderSuite) TestSingleEventBatchCompression(c *C) {
	accumulatedBatches := make(map[int][]byte) // A map of number -> event payload
	// We're using a map so we can add to it within the handler function and later access the data.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//	Log out information and the name of the test
		c.Assert(r.Header.Get("Content-Encoding"), Equals, "gzip")
		gz, err := gzip.NewReader(r.Body)
		c.Assert(err, IsNil)
		data, err := ioutil.ReadAll(gz)
		c.Assert(err, IsNil)

		accumulatedBatches[len(accumulatedBatches)] = data
	}))
	defer ts.Close()

	context := newTestContext("testAgent",
		&config.Config{
			PayloadCompressionLevel: gzip.BestCompression,
			CollectorURL:            ts.URL,
		})
	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "")

	// Waiting 2x the batch ticker should give plenty of time for it to process
	time.Sleep(EVENT_BATCH_TIMER_DURATION * 2 * time.Second)

	c.Assert(string(accumulatedBatches[0]), Equals, `[{"ExternalKeys":["testAgent"],"IsAgent":true,"Events":[{"eventType":"TestEvent","value":"5"}]}]`)
}

func (s *EventSenderSuite) TestLargeEventBatch(c *C) {
	accumulatedBatches := make(map[int][]byte) // A map of number -> event payload
	accumulatedRequests := make(map[int]*http.Request)
	// We're using a map so we can add to it within the handler function and later access the data.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		c.Assert(err, IsNil)
		accumulatedBatches[len(accumulatedBatches)] = body
		accumulatedRequests[len(accumulatedRequests)] = r
	}))
	defer ts.Close()

	context := newTestContext("testAgent",
		&config.Config{
			PayloadCompressionLevel: gzip.NoCompression,
			CollectorURL:            ts.URL,
		})
	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	for i := 0; i < MAX_EVENT_BATCH_COUNT+10; i++ {
		sender.QueueEvent(mapEvent{
			"eventType": "TestEvent",
			"value":     i,
			"entityID":  1,
		}, "")
	}

	// Waiting 2x the batch ticker should give plenty of time for it to process
	time.Sleep(EVENT_BATCH_TIMER_DURATION * 2 * time.Second)

	c.Assert(accumulatedBatches, HasLen, 2) // We should have made two event batch posts since we went over the max batch size

	var postedBatches []MetricPost
	c.Assert(json.Unmarshal(accumulatedBatches[0], &postedBatches), IsNil)
	c.Assert(postedBatches, HasLen, 1)
	c.Assert(postedBatches[0].Events, HasLen, MAX_EVENT_BATCH_COUNT)

	c.Assert(json.Unmarshal(accumulatedBatches[1], &postedBatches), IsNil)
	c.Assert(postedBatches, HasLen, 1)
	c.Assert(postedBatches[0].Events, HasLen, 10)
	c.Assert(postedBatches[0].EntityID, Equals, 1)

	c.Assert(accumulatedRequests, HasLen, 2)
	c.Assert(accumulatedRequests[0].Header.Get(http2.LicenseHeader), Equals, "license")
	c.Assert(accumulatedRequests[0].Header.Get("User-Agent"), Equals, "userAgent", http2.NullHttpClient, false)
	c.Assert(accumulatedRequests[0].Header.Get("Content-Type"), Equals, "application/json")
}

func (s *EventSenderSuite) TestLargeEventBatchCompression(c *C) {
	accumulatedBatches := make(map[int][]byte) // A map of number -> event payload
	accumulatedRequests := make(map[int]*http.Request)
	// We're using a map so we can add to it within the handler function and later access the data.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Header.Get("Content-Encoding"), Equals, "gzip")
		gz, err := gzip.NewReader(r.Body)
		c.Assert(err, IsNil)
		data, err := ioutil.ReadAll(gz)
		c.Assert(err, IsNil)
		accumulatedBatches[len(accumulatedBatches)] = data
		accumulatedRequests[len(accumulatedRequests)] = r
	}))
	defer ts.Close()

	context := newTestContext("testAgent",
		&config.Config{
			PayloadCompressionLevel: gzip.BestCompression,
			CollectorURL:            ts.URL,
		})
	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	for i := 0; i < MAX_EVENT_BATCH_COUNT+10; i++ {
		sender.QueueEvent(mapEvent{
			"eventType": "TestEvent",
			"value":     i,
		}, "")
	}

	// Waiting 2x the batch ticker should give plenty of time for it to process
	time.Sleep(EVENT_BATCH_TIMER_DURATION * 2 * time.Second)

	c.Assert(accumulatedBatches, HasLen, 2) // We should have made two event batch posts since we went over the max batch size

	var postedBatches []MetricPost
	c.Assert(json.Unmarshal(accumulatedBatches[0], &postedBatches), IsNil)
	c.Assert(postedBatches, HasLen, 1)
	c.Assert(postedBatches[0].Events, HasLen, MAX_EVENT_BATCH_COUNT)

	c.Assert(json.Unmarshal(accumulatedBatches[1], &postedBatches), IsNil)
	c.Assert(postedBatches, HasLen, 1)
	c.Assert(postedBatches[0].Events, HasLen, 10)

	c.Assert(accumulatedRequests, HasLen, 2)
	c.Assert(accumulatedRequests[0].Header.Get(http2.LicenseHeader), Equals, "license")
	c.Assert(accumulatedRequests[0].Header.Get("User-Agent"), Equals, "userAgent", http2.NullHttpClient, false)
	c.Assert(accumulatedRequests[0].Header.Get("Content-Type"), Equals, "application/json")
}

// Ensure the appropriate values for IsAgent and ExternalKeys are sent for remote entities
func (s *EventSenderSuite) TestBatchForRemoteEntity(c *C) {
	accumulatedBatches := make(map[int][]byte) // A map of number -> event payload
	// We're using a map so we can add to it within the handler function and later access the data.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		c.Assert(err, IsNil)
		accumulatedBatches[len(accumulatedBatches)] = body
	}))
	defer ts.Close()

	context := newTestContext("testAgent",
		&config.Config{
			PayloadCompressionLevel: gzip.NoCompression,
			CollectorURL:            ts.URL,
		})
	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "remoteEntity")

	// Waiting 2x the batch ticker should give plenty of time for it to process
	time.Sleep(EVENT_BATCH_TIMER_DURATION * 2 * time.Second)

	c.Assert(string(accumulatedBatches[0]), Equals, `[{"ExternalKeys":["remoteEntity"],"IsAgent":false,"Events":[{"eventType":"TestEvent","value":"5"}]}]`)
}

func (s *EventSenderSuite) TestEventKeyHeaderIsSent(c *C) {
	agentKey := "testAgentKey"
	agentKeyFromHeader := ""

	wg := sync.WaitGroup{}
	wg.Add(1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentKeyFromHeader = r.Header.Get(http2.EntityKeyHeader)
		wg.Done()
	}))
	defer ts.Close()

	ctx := newTestContext(agentKey,
		&config.Config{
			PayloadCompressionLevel: gzip.NoCompression,
			CollectorURL:            ts.URL,
		})
	sender := newMetricsIngestSender(ctx, "license", "userAgent", http2.NullHttpClient, false)

	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "")

	wg.Wait()
	c.Assert(agentKeyFromHeader, Equals, agentKey)
}

func (s *EventSenderSuite) TestAgentIDHeaderIsSentWhenConnectEnabled(c *C) {
	agentID := 123
	agentIDFromHeader := 0
	var agentIDErr error

	wg := sync.WaitGroup{}
	wg.Add(1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentIDFromHeader, agentIDErr = strconv.Atoi(r.Header.Get(http2.AgentEntityIdHeader))
		wg.Done()
	}))
	defer ts.Close()

	ctx := newTestContext(agentKey,
		&config.Config{
			PayloadCompressionLevel: gzip.NoCompression,
			CollectorURL:            ts.URL,
		})
	ctx.SetAgentIdentity(entity.Identity{ID: entity.ID(agentID)})
	sender := newMetricsIngestSender(ctx, "license", "userAgent", http2.NullHttpClient, true)

	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "")

	wg.Wait()
	c.Assert(agentIDFromHeader, Equals, agentID)
	c.Assert(agentIDErr, Equals, nil)
}

func (s *EventSenderSuite) TestValidSSLCert(c *C) {
	sca, err := ioutil.TempFile("", "server-ca.pem")
	c.Assert(err, IsNil)
	scert, err := ioutil.TempFile("", "server-cert.pem")
	c.Assert(err, IsNil)
	skey, err := ioutil.TempFile("", "server-key.key")
	c.Assert(err, IsNil)
	sca.WriteString(serverCA)
	scert.WriteString(serverCert)
	skey.WriteString(serverKey)

	cert, err := tls.LoadX509KeyPair(scert.Name(), skey.Name())
	if err != nil {
		log.Panic("bad server certs: ", err)
	}
	certs := []tls.Certificate{cert}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.TLS = &tls.Config{Certificates: certs}
	ts.StartTLS()

	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(u.Host)
	localhostURL := "https://localhost:" + port

	context := newTestContext("testAgent",
		&config.Config{
			CABundleFile: sca.Name(),
			CollectorURL: localhostURL,
		})

	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "")

	time.Sleep(EVENT_BATCH_TIMER_DURATION * 3 * time.Second)

	c.Assert(sender.sendErrorCount, Equals, uint32(0)) //The metric sender was unable to send the event
}

func (s *EventSenderSuite) TestMissingCACert(c *C) {
	scert, err := ioutil.TempFile("", "server-cert.pem")
	c.Assert(err, IsNil)
	skey, err := ioutil.TempFile("", "server-key.key")
	c.Assert(err, IsNil)
	scert.WriteString(serverCert)
	skey.WriteString(serverKey)

	cert, err := tls.LoadX509KeyPair(scert.Name(), skey.Name())
	if err != nil {
		log.Panic("bad server certs: ", err)
	}
	certs := []tls.Certificate{cert}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.TLS = &tls.Config{Certificates: certs}
	ts.StartTLS()

	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(u.Host)
	localhostURL := "https://localhost:" + port

	context := newTestContext("testAgent",
		&config.Config{
			CollectorURL: localhostURL,
		})

	sender := newMetricsIngestSender(context, "license", "userAgent", http2.NullHttpClient, false)
	c.Assert(sender.Start(), IsNil)
	defer sender.Stop()

	sender.QueueEvent(mapEvent{
		"eventType": "TestEvent",
		"value":     "5",
	}, "")

	time.Sleep(EVENT_BATCH_TIMER_DURATION * 5 * time.Second)

	c.Assert(sender.sendErrorCount, Equals, uint32(1)) //The metric sender was unable to send the event
}

var serverCA = `-----BEGIN CERTIFICATE-----
MIIDLzCCAhegAwIBAgIJAK0pSovZNG/IMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNV
BAMMCWxvY2FsaG9zdDAeFw0xNjEyMDcyMTU4MThaFw0yNjEyMDUyMTU4MThaMBQx
EjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBALHeiywOviXJeKriXkTDTvWtVKuJZNbQUQrl2Eyg50I1j+nWeKhiIh/bluPA
JI7Ll1GH9I+BDfAeWicyjkO3NVZAQGF6Svb7LpkQUF5oRMkfVlCd5ED5lw1F4j2p
H9Z3qKEWTgQlm0b0EtyWu4++78P0zNafxug8miJAIyhPCWhGNthP0n8wd4NtTsEe
VAXfmrIWQRqj60wGyioV4AXcgngyrfUQWYFLRkKIE4rtbMQnnFUPk2K2DBISOe5Z
yeYQ7NK9czqau1UDMtfHIEf+p5mAwDVhIqdfPCloWGwW0ntxKUUObt0zMx1QyRk2
Ml1qcXZGHCD4ALePBcdNyNQjfCcCAwEAAaOBgzCBgDAdBgNVHQ4EFgQUt33LKg5y
PEr6DwSSF7a5NhaxvL8wRAYDVR0jBD0wO4AUt33LKg5yPEr6DwSSF7a5NhaxvL+h
GKQWMBQxEjAQBgNVBAMMCWxvY2FsaG9zdIIJAK0pSovZNG/IMAwGA1UdEwQFMAMB
Af8wCwYDVR0PBAQDAgEGMA0GCSqGSIb3DQEBCwUAA4IBAQCuh+mHD9uolAWN7ghc
4q/1kSmIw9Ph4KtfN2ppeejPOQ1t/4OwPJVOmNpqF6iI33Ztc+dmZ7FAbC5dbjO1
JWO9W/JkUzfohx9KqsWPL4iMdbXu5sPBSkvCSjMbDCpSCxtR+eHVL7QjhvbJXOrD
hZ2wRXu4tKVloAjPqnQZMWMQolANAEyuqyV/Ot/1t/4lDID9/kTZRbzEDpUZytjM
7ESd6X2y6vqU5aFMhSCUqHap30cQ65OxgK+zOFEP/yNHtKctkEXN1IutANXKg6Ox
YacLM9wbhp+2CSzJO9URtn0ZpKHGjvvrJITiSY2cpdCSC05XuDPvonbKZw2rCDxZ
42z0
-----END CERTIFICATE-----
`

var serverCert = `-----BEGIN CERTIFICATE-----
MIIDSTCCAjGgAwIBAgIRANrfpJgunSjGgGcalvyeQq4wDQYJKoZIhvcNAQELBQAw
FDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTE2MTIwNzIxNTg0MFoXDTI2MTIwNTIx
NTg0MFowFDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAr0QEBIsOP4cQFgYG1v+ru9pL3583XNpKmc57ZKMoICknYGxB
HZ8SiyJ/XG4eAvUEEC/6sY1mtOcOoSt3DR9pv3csSsqd6wLGboEfbBUAWjvQ2fK3
ufZ4uKoN+G+nDH2ZcmlQdIrMGpo7EDZynevObvg1f69vKz5iGckRQaVar9ucXpiO
lHiyIApqlLUF3CvHjxGwt5FXZUFkzQYgmHaZX224eEw8cOBWz8WVxpdst5GZKKeR
4RMpFVVuFsV1wmA7uAR/3OBaOYuGWGTytdAC5+sfzU2V9y74sRq4RHuCJqKZoJ0d
726eva30XVwsKPMCK6vO5KJoFlv8tzHEZnUSHQIDAQABo4GVMIGSMAkGA1UdEwQC
MAAwHQYDVR0OBBYEFDhBYIBR4DxFnvG66QKF0JZjH095MEQGA1UdIwQ9MDuAFLd9
yyoOcjxK+g8Ekhe2uTYWsby/oRikFjAUMRIwEAYDVQQDDAlsb2NhbGhvc3SCCQCt
KUqL2TRvyDATBgNVHSUEDDAKBggrBgEFBQcDATALBgNVHQ8EBAMCBaAwDQYJKoZI
hvcNAQELBQADggEBADv/lXBntWsYJKajv7Nv3Pmnm6EebEPkGl+D6eaW1HK0htPy
j3CjMLt7CMvBy6QPoIluqyrnp3y5uUuyFtTzJ9n/t9r3p/7TAmXZWN+VxunC5QfA
5qjgopm4/b2v28vq9u77dpC6fehOHINv3LF2U70cLJL0WmnBl5/aVfAj9HLOeKWi
JIvgji9h0EFFw4h+CLbY+VCprTim9zNhiSJ/nI73jPJuaYwU/0uAOnRHFSn0wm3v
rLI/a16z15x+EQHAzilExhODb7jNXHyW2P+raBWtSLez8cdv/LwvmO4gi0RXHU0T
3XPBqNsKYS3qQm7aBhvGvJqhnLbmPQk77L2z59c=
-----END CERTIFICATE-----
`

var serverKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAr0QEBIsOP4cQFgYG1v+ru9pL3583XNpKmc57ZKMoICknYGxB
HZ8SiyJ/XG4eAvUEEC/6sY1mtOcOoSt3DR9pv3csSsqd6wLGboEfbBUAWjvQ2fK3
ufZ4uKoN+G+nDH2ZcmlQdIrMGpo7EDZynevObvg1f69vKz5iGckRQaVar9ucXpiO
lHiyIApqlLUF3CvHjxGwt5FXZUFkzQYgmHaZX224eEw8cOBWz8WVxpdst5GZKKeR
4RMpFVVuFsV1wmA7uAR/3OBaOYuGWGTytdAC5+sfzU2V9y74sRq4RHuCJqKZoJ0d
726eva30XVwsKPMCK6vO5KJoFlv8tzHEZnUSHQIDAQABAoIBAQCUISo8JMLwElkY
JBPX1tLwvDlwUQLbqWtvv0Iu9m7Nb7rmFdibDnz/tzJpjnAzE55RiBubwOTTdI26
zh+aqbgYqMJ4m/MIU5oef2dtU/quSOvlqOx7ccLqYF/aX5OSTP1J45SqSzziJwRQ
WZRZwJkC1SlyN3X/2IPVQ0kgcH7LLavIQltNHDdtJ2nJlQ39hCwMjL3sU/gCfI62
63B3t2j5QKC5FACJOhrMluPnujnRpEmoGBBGk/J1N60IluH0fBBmKCiBmZRuRokR
+6x+3X57Vsyv7gQazLwjXkbZQILceMl7wOc35FN7wcdm42fDJA9ksUWw9icWd7Z8
Cxc8i12BAoGBAOiImolwTpiI08OY76P5QpYBzqJaNgrleevOgfY9kiD1hCrtL5O6
YIaNH6+iqF0GkXBrYiPXEN834cjUOPCFLFkq6LA6mnh1fFmeWuR/YjFWRamEiGNc
dpbgYOiWSZQIsJ98ATelEMpDK0IZNk42h32MYUj0odBnZzii2ivsQBjxAoGBAMDz
7BbyYYWToeHemDFRccmWGFlHVslSZmaORWxPyEPeztuPb0B8eX2lxvq2sWiwWmN9
U14N1bFgzUnNlzjTWtCP70ENpUZwH5giYt/hqA5VB83Tlizz6qTnjjL+yt+ShGeC
mHc5wgrLEkobWjgrsZ5fC558vsmYf/XIe3t9xavtAoGAeyCKI6hSFbat2u5KRhsg
ROnkDleSMibcbh5E9qwXilg2ibxZ6vFPVy+2zXtlhwyJSmux5aRljKy8Y2jsVX9O
wlBUMax2Sr56/8E4A7HnvpQeAurohSRarv1UkxOxFi+bxncU9e/zegYjC7bp6HQ7
PiFtCgJvBDkckQK6X3OTZSECgYEAlE9pGsI3X3r4pSp5hP77HV23XXhOJvSlLRMS
HYy9fB2Yln8Lnl+O6psv9KnUd4mGEI7WM6cip/KmGKJkKBOc7E6WMkSQ0zF/t2pG
4ZsLl+iX1QdbmTXrF9G8JUpfGbX++6kQFCRbF/y7FCcuE5rSLc8UmT99Tmtff8YX
0/X6qIkCgYBoUmxF0F2C/sI4zJ/UJjonD2XJ3tWPHIjeHIXqppcYAH6QNawDAa8k
nchZhPmtpZbxThD0YLYabEy7GKbyN0CPvqB6E3MruI9YlQPKPAR/gA6w8bT5X9YJ
TdqiXoKIOldFj+KiyLZmEJ0vxZ4PjHCptMMhPBlsK+q+Uoh3AAer3A==
-----END RSA PRIVATE KEY-----
`

func TestEventSender_ResponseError(t *testing.T) {
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

			cfg := &config.Config{
				ConnectEnabled:          true,
				PayloadCompressionLevel: gzip.NoCompression,
			}
			c := NewContext(cfg, "1.2.3", testhelpers.NullHostnameResolver, host.IDLookup{}, nil, nil)
			c.setAgentKey(agentKey)
			c.SetAgentIdentity(agentIdn)

			sender := newMetricsIngestSender(
				c,
				"license",
				"userAgent",
				rc.Client,
				true,
			)

			backoffCh := make(chan time.Duration)
			sender.getBackoffTimer = func(d time.Duration) *time.Timer {
				backoffCh <- d
				return time.NewTimer(0)
			}
			assert.NoError(t, sender.Start())
			defer sender.Stop()

			assert.NoError(t, sender.QueueEvent(ev, ""))

			<-rc.RequestCh

			assert.Equal(t, test.backoffD, <-backoffCh)
			assert.Equal(t, uint32(1), sender.sendErrorCount, "The metric sender was unable to send the event")
		})
	}
}

func newTestContext(agentKey string, cfg *config.Config) *context {
	var atomicAgentKey atomic.Value
	atomicAgentKey.Store(agentKey)
	return &context{
		agentKey: atomicAgentKey,
		cfg:      cfg,
	}
}
