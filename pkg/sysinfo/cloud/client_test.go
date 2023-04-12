// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHttpClientTimeout(t *testing.T) {
	t.Parallel()

	testURI := "/valid"
	mux := http.NewServeMux()
	mux.Handle(testURI, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(defaultDialTimeout * 2)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("i-db519dd1\n"))
		return
	}))

	server := httptest.NewServer(mux)

	// create default fast client
	client := clientWithFastTimeout(false)

	resp, err := client.Get(fmt.Sprintf("%s/%s", server.URL, testURI))
	defer resp.Body.Close()

	// should fail as server hangs more time than the defaultDialTimeout
	assert.Error(t, err)
}
