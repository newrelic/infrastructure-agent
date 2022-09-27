// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fwrequest

import (
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommonAttributes(t *testing.T) {

	id := entity.ID(115)
	agentVersion := "agentVersion"
	intVersion := "intVersion"
	intName := "intName"
	fw := NewEntityFwRequest(protocol.Dataset{}, id, FwRequestMeta{}, protocol.IntegrationMetadata{Name: intName, Version: intVersion}, agentVersion)

	assert.Equal(t, agentVersion, fw.Data.Common.Attributes[CollectorVersionAttribute])
	assert.Equal(t, agentCollector, fw.Data.Common.Attributes[CollectorNameAttribute])
	assert.Equal(t, intVersion, fw.Data.Common.Attributes[InstrumentationVersionAttribute])
	assert.Equal(t, intName, fw.Data.Common.Attributes[InstrumentationNameAttribute])
	assert.Equal(t, newRelicProvider, fw.Data.Common.Attributes[InstrumentationProviderAttribute])
	assert.Equal(t, id.String(), fw.Data.Common.Attributes[EntityIdAttribute])

}

func TestCommonAttributesEmptyEntity(t *testing.T) {

	agentVersion := "agentVersion"
	intVersion := "intVersion"
	intName := "intName"
	fw := NewEntityFwRequest(protocol.Dataset{}, entity.EmptyID, FwRequestMeta{}, protocol.IntegrationMetadata{Name: intName, Version: intVersion}, agentVersion)
	// No Panic expected

	assert.Equal(t, agentVersion, fw.Data.Common.Attributes[CollectorVersionAttribute])
	assert.Equal(t, agentCollector, fw.Data.Common.Attributes[CollectorNameAttribute])
	assert.Equal(t, intVersion, fw.Data.Common.Attributes[InstrumentationVersionAttribute])
	assert.Equal(t, intName, fw.Data.Common.Attributes[InstrumentationNameAttribute])
	assert.Equal(t, newRelicProvider, fw.Data.Common.Attributes[InstrumentationProviderAttribute])
	assert.NotContains(t, fw.Data.Common.Attributes, EntityIdAttribute)

}
