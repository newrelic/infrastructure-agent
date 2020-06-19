// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testhelp

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

// GoRun returns the command path to run a Go file as if it were a command.
// It expects to find the "go" executable in the path
func GoRun(gofile Script, args ...string) config.ShlexOpt {
	return append(config.ShlexOpt{GoCommand(), "run", string(gofile)}, args...)
}

// GoCommand returns the complete path to the Go command
func GoCommand() string {
	gocmd, err := getGoCommand()
	if err != nil {
		return "cant-find-go:" + err.Error()
	}

	if runtime.GOOS == "windows" {
		gocmd = strings.Replace(gocmd, `\`, `\\`, -1)
	}
	return gocmd
}

func getGoCommand() (string, error) {
	lookPath, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		lookPath = strings.Replace(lookPath, `\`, `\\`, -1)
	}
	return lookPath, err
}

func GetGoEnv(env string) (string, error) {
	goCommand, err := getGoCommand()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(goCommand, "env", env)
	bs, err := cmd.Output()
	if err != nil {
		return "", err
	}
	val := string(bs)
	if runtime.GOOS == "windows" {
		val = strings.Replace(val, `\`, `\\`, -1)
	}
	return strings.TrimSpace(val), err
}

// GoBuild will compile a simple Go source file and place the executable in the destination path
func GoBuild(gofile Script, dstPath string) error {
	cmd := exec.Command(GoCommand(), "build", "-o", dstPath, strings.TrimSuffix(strings.TrimPrefix(string(gofile), "'"), "'"))
	if err := cmd.Run(); err != nil {
		var output string
		if bytes, err2 := cmd.CombinedOutput(); err2 == nil {
			output = string(bytes)
		} else {
			output = err2.Error()
		}
		return fmt.Errorf("can't compile %q into %q: %s\nOutput:\n%s\n<END>",
			gofile, dstPath, err.Error(), output)
	}
	return nil
}

type Script string

func WrapScript(scriptPath string) Script {
	return Script(fmt.Sprintf("'%s'", scriptPath))
}

func WrapScriptPath(elem ...string) Script {
	return WrapScript(path.Join(elem...))
}

func Command(script Script, args ...string) config.ShlexOpt {
	return append(append(getDefaultShlexOpt(), string(script)), args...)
}

func CommandSlice(script Script, args ...string) []string {
	return append(append(getDefaultShlexOpt(), string(script)), args...)
}

func getDefaultShlexOpt() config.ShlexOpt {
	if runtime.GOOS == "windows" {
		return config.ShlexOpt{"powershell.exe", "-Sta", "-File"}
	}
	return config.ShlexOpt{"/bin/sh"}
}
