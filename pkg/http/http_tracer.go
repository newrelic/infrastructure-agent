// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"time"

	wlog "github.com/newrelic/infrastructure-agent/pkg/log"
)

var tlog = wlog.WithComponent("HttpTracer")

func WithTracer(req *http.Request, requester string) *http.Request {
	l := tlog.WithField("requester", requester)
	traceStart := time.Now()
	var getConnStart, dnsStart, conStart, tlsStart time.Time
	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			getConnStart = time.Now()
			l.WithField("action", "GetConn").
				WithField("hostPort", hostPort).
				Debug("")
		},
		GotConn: func(info httptrace.GotConnInfo) {
			l.WithField("action", "GotConn").
				WithField("wasIdle", info.WasIdle).
				WithField("idleTime", info.IdleTime).
				WithField("duration", fmt.Sprintf("%dms", time.Since(getConnStart).Milliseconds())).
				Debug("")
		},
		GotFirstResponseByte: func() {
			l.WithField("action", "GotFirstResponseByte").
				WithField("duration", fmt.Sprintf("%dms", time.Since(traceStart).Milliseconds())).
				Debug("")
		},
		Got100Continue: func() {
			l.WithField("action", "Got100Continue").
				Debug("")
		},
		Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
			l.WithField("action", "Got1xxResponse").
				WithField("code", code).Debug("")

			return nil
		},
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
			l.WithField("action", "DNSStart").
				Debug("")

		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			l.WithField("action", "DNSDone").
				WithField("duration", fmt.Sprintf("%dms", time.Since(dnsStart).Milliseconds())).
				Debug("")
		},
		ConnectStart: func(network, addr string) {
			conStart = time.Now()
			l.WithField("action", "ConnectStart").
				WithField("network", network).
				WithField("addr", addr).
				Debug("")
		},
		ConnectDone: func(network, addr string, err error) {
			l.WithField("action", "ConnectDone").
				WithField("network", network).
				WithField("addr", addr).
				WithError(err).
				WithField("duration", fmt.Sprintf("%dms", time.Since(conStart).Milliseconds())).
				Debug("")
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
			l.WithField("action", "TLSHandshakeStart").
				Debug("")
		},
		TLSHandshakeDone: func(tlsState tls.ConnectionState, err error) {
			tlsVersion := ""
			switch tlsState.Version {
			case tls.VersionTLS10:
				{
					tlsVersion = "1.0"
				}
			case tls.VersionTLS11:
				{
					tlsVersion = "1.1"
				}
			case tls.VersionTLS12:
				{
					tlsVersion = "1.2"
				}
			case tls.VersionTLS13:
				{
					tlsVersion = "1.3"
				}
			}
			l.WithField("action", "TLSHandshakeDone").
				WithField("duration", fmt.Sprintf("%dms", time.Since(tlsStart).Milliseconds())).
				WithField("version", tlsVersion).
				Debug("")
		},
		WroteHeaders: func() {
			l.WithField("action", "WroteHeaders").
				Debug("")
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			l.WithField("action", "WroteRequest").
				WithError(info.Err).
				Debug("")
		},
	}

	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}
