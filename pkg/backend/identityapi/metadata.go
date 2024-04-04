// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import (
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid"
)

type Metadata map[string]string

const hostIDAttribute = "host.id"

var errUnableRetrieveHostID = fmt.Errorf("unable to retrieve host.id")

type MetadataHarvester interface {
	Harvest() (Metadata, error)
}

type MetadataHarvesterDefault struct {
	hostIDProvider hostid.Provider
}

func NewMetadataHarvesterDefault(hostIDProvider hostid.Provider) *MetadataHarvesterDefault {
	return &MetadataHarvesterDefault{
		hostIDProvider: hostIDProvider,
	}
}

func (h *MetadataHarvesterDefault) Harvest() (Metadata, error) {
	metadata := make(Metadata)

	// get host.it from env var
	hostID, err := h.hostIDProvider.Provide()
	if err != nil {
		return nil, errors.Join(errUnableRetrieveHostID, err)
	}

	metadata[hostIDAttribute] = hostID

	return metadata, nil
}
