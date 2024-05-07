// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"os/exec"
	"strings"
)

// Make mocking simpler
var cyberArkExecCommand = exec.Command

type CyberArkCLI struct {
	CLI    string `yaml:"cli"`
	AppID  string `yaml:"app-id"`
	Safe   string `yaml:"safe"`
	Folder string `yaml:"folder"`
	Object string `yaml:"object"`
}

type cyberArkCLIGatherer struct {
	cfg *CyberArkCLI
}

// CyberArkCLIGatherer instantiates a CyberArkCLI variable gatherer from the given configuration.
// The result is a map with a single "password" key value pair
func CyberArkCLIGatherer(cyberArkCLI *CyberArkCLI) func() (interface{}, error) {
	g := cyberArkCLIGatherer{cfg: cyberArkCLI}
	return func() (interface{}, error) {
		dt, err := g.get()
		if err != nil {
			return "", err
		}
		return dt, err
	}
}

func (g *cyberArkCLIGatherer) get() (data.InterfaceMap, error) {
	cmd := g.cyberArkExecCommand()
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve cyberArkCLI secret from cli. err: %s err msg: %s", err, stderr.String())
	}
	// End-of-line fixup from CLI
	password := strings.TrimSuffix(out.String(), "\n")
	password = strings.TrimSuffix(password, "\r")

	if password == "" {
		return nil, fmt.Errorf("empty password returned from cyberArkCLI")
	}
	result := data.InterfaceMap{}
	result["password"] = password
	return result, nil
}

func (g *CyberArkCLI) Validate() error {
	if g.CLI == "" || g.AppID == "" || g.Safe == "" || g.Folder == "" || g.Object == "" {
		return errors.New("cyberArkCLI secrets must have cli, app-id, safe, folder, and object in order to be set")
	}
	return nil
}
