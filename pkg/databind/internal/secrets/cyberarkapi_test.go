// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	. "net/http"
	"net/http/httptest"
	"testing"
)

func TestCyberArkAPI(t *testing.T) {

	ts := newHttpTestServer(
		`{
  "Content": "CI<s0H9>k2zUPXM",
  "PolicyID": "ALL-APPX-WIN-SL-XNCVRRI-DEFAULT",
  "SequenceID": "2",
  "UserName": "testuser",
  "CPMStatus": "success",
  "Folder": "Root",
  "Safe": "ALL-NERE-WIN-A-NEWRELIC-UP",
  "Address": "localhost",
  "Name": "ALL-localhost-testuser",
  "LogonDomain": "localhost",
  "DeviceType": "Operating System",
  "LastTask": "ChangeTask",
  "RetriesCount": "-1",
  "LastSuccessChange": "1589288852",
  "CreationMethod": "PVWA",
  "PasswordChangeInProcess": "False"
}`, 200)

	defer ts.Close()
	apiStruct := CyberArkAPI{
		HTTP: &(http{
			URL:     ts.URL,
			Headers: make(map[string]string),
		}),
	}

	g := CyberArkAPIGatherer(&apiStruct)

	r, err := g()
	if err != nil {
		t.Errorf("api call failed: %v ", err)
	}

	unboxed := r.(data.InterfaceMap)

	if unboxed == nil {
		t.Errorf("Result is nil")
	}

	if unboxed["password"] != "CI<s0H9>k2zUPXM" {
		t.Errorf("expected password, got %v", unboxed)
	}

	if unboxed["user"] != "testuser" {
		t.Errorf("expected user, got %v", unboxed)
	}
}

func TestCyperArkAPIResponeCodes(t *testing.T) {
	//Bad request 400 The request could not be understood by the server due to incorrect syntax.
	//Unauthorized 401 The request requires user authentication.
	//Forbidden 403 The server received and understood the request, but will not fulfill it. Authorization will not help and the request MUST NOT be repeated.
	//Not Found 404 The server did not find anything that matches the Request-URI. No indication is given of whether the condition is temporary or permanent.
	//Conflict 409 The request could not be completed due to a conflict with the current state of the resource.
	//Internal Server Error 500 The server encountered an unexpected condition which prevented it from fulfilling the request.
	codes := []int{400, 401, 403, 404, 409, 500}
	for _, rc := range codes {
		ts := newHttpTestServer("", rc)
		defer ts.Close()
		apiStruct := CyberArkAPI{
			HTTP: &(http{
				URL:     ts.URL,
				Headers: make(map[string]string),
			}),
		}

		g := CyberArkAPIGatherer(&apiStruct)

		_, err := g()
		if err == nil {
			t.Errorf("api call should have filed with %d Error: %v ", rc, err)
		}
	}
}

func newHttpTestServer(response string, rc int) *httptest.Server {
	return httptest.NewServer(HandlerFunc(func(w ResponseWriter, r *Request) {
		w.WriteHeader(rc)
		w.Write([]byte(response))
	}))
}
