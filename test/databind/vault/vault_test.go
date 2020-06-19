// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package vault_test

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	test "github.com/newrelic/infrastructure-agent/test/databind"
)

// values set in the corresponding docker-compose.yml
const (
	tokenHeader = "X-Vault-Token"
	token       = "my_root_token"
	host        = "http://localhost:11234"
	v1Host      = "http://localhost:11235"
	secretUrl   = host + "/v1/secret/data/mysecret"
	v1SecretUrl = v1Host + "/v1/secret/data/mysecret"
	healthUrl   = host + "/v1/sys/health"
	v1HealthUrl = v1Host + "/v1/sys/health"
)

func TestMain(m *testing.M) {
	if err := test.ComposeUp("./docker-compose.yml"); err != nil {
		log.Println("error on compose-up: ", err.Error())
		os.Exit(-1)
	}
	var exitVal int
	func() {
		defer test.ComposeDown("./docker-compose.yml")
		exitVal = m.Run()
	}()

	os.Exit(exitVal)
}

func TestVault_Secret(t *testing.T) {
	require.NoError(t, waitForConnection(v1HealthUrl))
	// GIVEN a secret stored in vault
	req, err := http.NewRequest(http.MethodPost, v1SecretUrl, bytes.NewBufferString(
		`{"name":{"first":"Pep","second":"Guardiola"},"password":"u1tr4s3cr3t","salt":[1,2,3,4]}`))
	require.NoError(t, err)
	req.Header.Set(tokenHeader, token)
	req.Header.Set("X-Vault-Request", "true")
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	require.Equalf(t, http.StatusNoContent, resp.StatusCode,
		"unexpected status: %v - %v", resp.StatusCode, resp.Status)

	// WHEN the secret is retrieved
	input := fmt.Sprintf(`
variables:
  secret:
    vault:
      http:
        url: %s
        headers:
          %s: %s
`, v1SecretUrl, tokenHeader, token)
	ctx, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)
	templ := map[string]string{
		"fn": "${secret.name.first}",
		"sn": "${secret.name.second}",
		"p":  "${secret.password}",
		"s0": "${secret.salt[0]}",
		"s1": "${secret.salt[1]}",
		"s2": "${secret.salt[2]}",
		"s3": "${secret.salt[3]}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	assert.Equal(t, "Pep", d["fn"])
	assert.Equal(t, "Guardiola", d["sn"])
	assert.Equal(t, "u1tr4s3cr3t", d["p"])
	assert.Equal(t, "1", d["s0"])
	assert.Equal(t, "2", d["s1"])
	assert.Equal(t, "3", d["s2"])
	assert.Equal(t, "4", d["s3"])
}

func TestVault_TokenAuth(t *testing.T) {
	require.NoError(t, waitForConnection(healthUrl))

	// GIVEN a secret stored in vault
	req, err := http.NewRequest(http.MethodPost, secretUrl, bytes.NewBufferString(
		`{"data":{"name":{"first":"Pep","second":"Guardiola"},"password":"u1tr4s3cr3t","salt":[1,2,3,4]}}`))
	require.NoError(t, err)
	req.Header.Set(tokenHeader, token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, resp.StatusCode,
		"unexpected status: %v - %v", resp.StatusCode, resp.Status)

	// WHEN the secret is retrieved
	input := fmt.Sprintf(`
variables:
  secret:
    vault:
      http:
        url: %s
        headers:
          %s: %s
`, secretUrl, tokenHeader, token)
	ctx, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)
	templ := map[string]string{
		"fn": "${secret.name.first}",
		"sn": "${secret.name.second}",
		"p":  "${secret.password}",
		"s0": "${secret.salt[0]}",
		"s1": "${secret.salt[1]}",
		"s2": "${secret.salt[2]}",
		"s3": "${secret.salt[3]}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	assert.Equal(t, "Pep", d["fn"])
	assert.Equal(t, "Guardiola", d["sn"])
	assert.Equal(t, "u1tr4s3cr3t", d["p"])
	assert.Equal(t, "1", d["s0"])
	assert.Equal(t, "2", d["s1"])
	assert.Equal(t, "3", d["s2"])
	assert.Equal(t, "4", d["s3"])
}

func waitForConnection(url string) error {
	const timeout = 5 * time.Second
	now := time.Now()
	var resp *http.Response
	var err error
	for now.Add(timeout).Unix() > time.Now().Unix() {
		resp, err = http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return nil
		}
	}
	if resp != nil {
		return fmt.Errorf("can't connect to vault: %v - %v", resp.StatusCode, resp.Status)
	}
	return fmt.Errorf("can't connect to vault: %s", err)
}
