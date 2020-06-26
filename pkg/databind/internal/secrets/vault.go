// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

type Vault struct {
	HTTP *http
}

type vaultGatherer struct {
	cfg *Vault
}

// VaultGatherer instantiates a Vault variable gatherer from the given configuration. The fetching process
// will return either a map containing access paths to the stored JSON.
// E.g. if the stored secret is `{"person":{"name":"Matias","surname":"Burni"}}`, the returned Map
// contents will be:
// "person.name"    -> "Matias"
// "person.surname" -> "Burni"
func VaultGatherer(vault *Vault) func() (interface{}, error) {
	g := vaultGatherer{cfg: vault}
	return func() (interface{}, error) {
		dt, err := g.get()
		if err != nil {
			return "", err
		}
		return dt, err
	}
}

func (g *vaultGatherer) get() (data.InterfaceMap, error) {
	secret := g.cfg
	dt, err := httpRequest(secret.HTTP, "GET", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve vault secret from http server: %s", err)
	}

	smap := data.InterfaceMap{}
	if err := json.Unmarshal(dt, &smap); err != nil {
		return nil, fmt.Errorf("unable to decode vault secret: %s", err)
	}
	if d, ok := smap["data"]; ok {
		if sdata, ok := d.(map[string]interface{})["data"]; ok {
			if idata, ok := sdata.(map[string]interface{}); ok {
				return idata, nil
			}
		}
		if idata, ok := d.(map[string]interface{}); ok {
			return idata, nil
		}
	}
	return nil, fmt.Errorf("vault returned an unexpected format from the http server: %s", string(dt))
}

func (g *Vault) Validate() error {
	if g.HTTP == nil {
		return errors.New("vault secrets must have an http parameter with a URL in order to be set")
	}
	if g.HTTP.URL == "" {
		return errors.New("vault secrets must have an http URL parameter in order to be set")
	}
	return nil
}
