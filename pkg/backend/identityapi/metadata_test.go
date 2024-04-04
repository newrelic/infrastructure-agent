// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import (
	"errors"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataHarvesterDefaultHostIdeProvided(t *testing.T) {
	t.Parallel()

	hostIDProvider := &hostid.ProviderMock{} //nolint:exhaustruct
	defer hostIDProvider.AssertExpectations(t)

	harvester := NewMetadataHarvesterDefault(hostIDProvider)

	hostIDProvider.ShouldProvide("some-host-id")

	metadata, err := harvester.Harvest()
	require.NoError(t, err)
	assert.Equal(t, "some-host-id", metadata[hostIDAttribute])
}

func TestMetadataHarvesterDefaultErrorOnHostId(t *testing.T) {
	t.Parallel()

	hostIDProvider := &hostid.ProviderMock{} //nolint:exhaustruct
	defer hostIDProvider.AssertExpectations(t)

	harvester := NewMetadataHarvesterDefault(hostIDProvider)

	//nolint:goerr113
	providerErr := errors.New("some error")
	hostIDProvider.ShouldReturnErr(providerErr)

	metadata, err := harvester.Harvest()
	require.ErrorAs(t, err, &providerErr)
	assert.NotContains(t, metadata, hostIDAttribute)
}
