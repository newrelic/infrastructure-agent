// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

type CyberArkAPI struct {
	HTTP *http
}

type cyberArkAPIGatherer struct {
	cfg *CyberArkAPI
}

// CyberArkAPIGatherer instantiates a CyberArkAPI variable gatherer from the given configuration.
// The result is a map with a single "password" key value pair
func CyberArkAPIGatherer(cyberArkAPI *CyberArkAPI) func() (interface{}, error) {
	g := cyberArkAPIGatherer{cfg: cyberArkAPI}
	return func() (interface{}, error) {
		dt, err := g.get()
		if err != nil {
			return "", err
		}
		return dt, err
	}
}

func (g *cyberArkAPIGatherer) get() (data.InterfaceMap, error) {
	secret := g.cfg
	dt, err := httpRequest(secret.HTTP, "GET", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve cyberarkapi secret from http server: %s", err)
	}

	smap := data.InterfaceMap{}
	if err := json.Unmarshal(dt, &smap); err != nil {
		return nil, fmt.Errorf("unable to decode cyberarkapi secret: %s", err)
	}
	if p, ok := smap["Content"]; ok {
		if u, ok := smap["UserName"]; ok {
			result := data.InterfaceMap{}
			result["password"] = p.(string)
			result["user"] = u.(string)
			return result, nil
		}
	}
	return nil, fmt.Errorf("cyberarkapi returned an unexpected format from the http server: %s", string(dt))
}

func (g *CyberArkAPI) Validate() error {
	if g.HTTP == nil {
		return errors.New("cyberarkapi secrets must have an http parameter with a URL in order to be set")
	}
	if g.HTTP.URL == "" {
		return errors.New("cyberarkapi secrets must have an http URL parameter in order to be set")
	}
	return nil
}
