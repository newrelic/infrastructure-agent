// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
//nolint:godoclint // package doc convention is inconsistent repo-wide; not fixing that here
package cloud

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errInstancePrincipalUnavailable stands in for the real OCI SDK/network error that occurs
// when instance principal metadata isn't reachable (e.g. running outside OCI).
var errInstancePrincipalUnavailable = errors.New("instance principal metadata not available")

// testRSAKeyOnce is a throwaway key generated once, used only to let the SDK's request signer
// construct successfully in tests - it's never used to sign anything sent over the network.
var testRSAKeyOnce = sync.OnceValue(func() *rsa.PrivateKey { //nolint:gochecknoglobals
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	return key
})

// fakeConfigurationProvider is a minimal common.ConfigurationProvider that lets us get past
// SDK client construction in initAPIClients without a real instance principal cert exchange.
type fakeConfigurationProvider struct{}

func (fakeConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return testRSAKeyOnce(), nil
}

func (fakeConfigurationProvider) KeyID() (string, error) {
	return "fake-key-id", nil
}

func (fakeConfigurationProvider) TenancyOCID() (string, error) {
	return "ocid1.tenancy.oc1..fake", nil
}

func (fakeConfigurationProvider) UserOCID() (string, error) {
	return "", nil
}

func (fakeConfigurationProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (fakeConfigurationProvider) Region() (string, error) {
	return "us-ashburn-1", nil
}

func (fakeConfigurationProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{AuthType: common.InstancePrincipal, IsFromConfigFile: false, OboToken: nil}, nil
}

// newHarvesterWithFailingAuth returns an OCIHarvester whose instance principal auth always
// fails, exercising the Phase 2 graceful-degradation path without any network calls.
func newHarvesterWithFailingAuth(t *testing.T) (*OCIHarvester, *int) {
	t.Helper()

	callCount := 0

	orig := newInstancePrincipalProvider
	newInstancePrincipalProvider = func() (common.ConfigurationProvider, error) {
		callCount++

		return nil, errInstancePrincipalUnavailable //nolint:wrapcheck
	}

	t.Cleanup(func() { newInstancePrincipalProvider = orig })

	return &OCIHarvester{ //nolint:exhaustruct
		timeout: NewTimeout(ociTimeout),
	}, &callCount
}

// TestInitAPIClients_ProviderError - no t.Parallel(): overrides the shared package-level
// newInstancePrincipalProvider var, which is unsafe to mutate concurrently with other tests in
// this file (a previous attempt at parallelizing let a real, hanging IMDS call slip through).
func TestInitAPIClients_ProviderError(t *testing.T) { //nolint:paralleltest
	harvester, callCount := newHarvesterWithFailingAuth(t)

	err := harvester.initAPIClients()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOCIAPIUnavailable)
	assert.Equal(t, 1, *callCount)

	// A second call must not re-invoke the provider - sync.Once caches the failure.
	err2 := harvester.initAPIClients()
	require.Error(t, err2)
	assert.Equal(t, 1, *callCount)
	assert.Equal(t, err, err2)
}

// TestInitAPIClients_Success - no t.Parallel(), see TestInitAPIClients_ProviderError.
func TestInitAPIClients_Success(t *testing.T) { //nolint:paralleltest
	orig := newInstancePrincipalProvider
	newInstancePrincipalProvider = func() (common.ConfigurationProvider, error) {
		return fakeConfigurationProvider{}, nil
	}

	t.Cleanup(func() { newInstancePrincipalProvider = orig })

	harvester := &OCIHarvester{timeout: NewTimeout(ociTimeout)} //nolint:exhaustruct

	err := harvester.initAPIClients()
	require.NoError(t, err)
	assert.NotNil(t, harvester.computeClient)
	assert.NotNil(t, harvester.vnClient)
}

// TestPhase2Getters_AuthUnavailable - no t.Parallel(), see TestInitAPIClients_ProviderError.
func TestPhase2Getters_AuthUnavailable(t *testing.T) { //nolint:paralleltest
	cases := []struct {
		name string
		call func(*OCIHarvester) (string, error)
	}{
		{"GetVCNID", (*OCIHarvester).GetVCNID},
		{"GetSubnetID", (*OCIHarvester).GetSubnetID},
		{"GetLifecycleState", (*OCIHarvester).GetLifecycleState},
		{"GetVirtualizationType", (*OCIHarvester).GetVirtualizationType},
		{"GetDedicatedVMHostID", (*OCIHarvester).GetDedicatedVMHostID},
	}

	for _, tc := range cases { //nolint:paralleltest
		t.Run(tc.name, func(t *testing.T) {
			harvester, _ := newHarvesterWithFailingAuth(t)

			value, err := tc.call(harvester)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrOCIAPIUnavailable)
			assert.Empty(t, value)
		})
	}
}

// TestGetPrimaryVnic_NoVnicsFound - no t.Parallel(), see TestInitAPIClients_ProviderError.
func TestGetPrimaryVnic_NoVnicsFound(t *testing.T) { //nolint:paralleltest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	origEndpoint := ociVnicsEndpoint
	ociVnicsEndpoint = server.URL + "/"

	t.Cleanup(func() { ociVnicsEndpoint = origEndpoint })

	origProvider := newInstancePrincipalProvider
	newInstancePrincipalProvider = func() (common.ConfigurationProvider, error) {
		return fakeConfigurationProvider{}, nil
	}

	t.Cleanup(func() { newInstancePrincipalProvider = origProvider })

	harvester := &OCIHarvester{timeout: NewTimeout(ociTimeout)} //nolint:exhaustruct

	_, err := harvester.getPrimaryVnic()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOCIAPIUnavailable)
	assert.Contains(t, err.Error(), "no VNICs found in IMDS")
}
