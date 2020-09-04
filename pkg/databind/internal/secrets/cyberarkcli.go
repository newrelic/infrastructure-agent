// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"os"
	"os/exec"
)

// Make mocking simpler
var cyberArkExecCommand = exec.Command

type CyberArkCLI struct {
	CLI    string `yaml:"cli"`
	AppID  string `yaml:"app-id"`
	Safe   string
	Folder string
	Object string
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
	// GetPassword -p AppDescs.AppID=$APP_ID -p Query=\"Safe=$SAFE;Folder=$FOLDER;Object=$OBJECT\"; -o Password
	r, err := cyberArkExecCommand(g.cfg.CLI, "GetPassword", "-p", "AppDescs.AppID="+g.cfg.AppID, "-p", "Query=\"Safe="+g.cfg.Safe+";Folder="+g.cfg.Folder+";Object="+g.cfg.Object+"\"; -o Password").Output()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve cyberArkCLI secret from cli: %s", err)
	}
	password := string(r)
	if password == "" {
		return nil, fmt.Errorf("empty password returned from cyberArkCLI")
	}
	result := data.InterfaceMap{}
	result["password"] = password
	fmt.Fprintf(os.Stderr, "get: returning %v\n\n", result)
	return result, nil
}

func (g *CyberArkCLI) Validate() error {
	if g.CLI == "" || g.AppID == "" || g.Safe == "" || g.Folder == "" || g.Object == "" {
		return errors.New("cyberArkCLI secrets must have cli, app-id, safe, folder, and object in order to be set")
	}
	return nil
}
