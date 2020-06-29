// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
)

// Container discovery parameters
type Container struct {
	Match      map[string]string `yaml:"match"`
	ApiVersion string            `yaml:"api_version"` // for docker client
}

func (d *Container) Validate() error {
	if len(d.Match) == 0 {
		return errors.New("missing 'match' entries")
	}
	return nil
}
