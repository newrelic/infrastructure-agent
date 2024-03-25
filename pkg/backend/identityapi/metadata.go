// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import "os"

const hostIDEnv = "NR_HOST_ID"

type Metadata map[string]string

type MetadataHarvester interface {
	Harvest() (Metadata, error)
}

type MetadataHarvesterDefault struct{}

func NewMetadataHarvesterDefault() *MetadataHarvesterDefault {
	return &MetadataHarvesterDefault{}
}

func (h *MetadataHarvesterDefault) Harvest() (Metadata, error) {
	metadata := make(Metadata)

	// get host.it from env var
	hostID, exists := os.LookupEnv(hostIDEnv)
	if exists {
		metadata["host.id"] = hostID
	}

	return metadata, nil
}
