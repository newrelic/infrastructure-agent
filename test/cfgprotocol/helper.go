// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cfgprotocol

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"text/template"

	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/assert"
)

func findAllProcessByCmd(re *regexp.Regexp) ([]*process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, err
	}
	return findProcessByCmd(re, ps), nil
}

func findChildrenProcessByCmdName(re *regexp.Regexp) ([]*process.Process, error) {
	pp, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return nil, err
	}
	children, err := pp.Children()
	if err != nil {
		return nil, err
	}
	pFound := make([]*process.Process, 0)
	for _, p := range children {
		c, err := p.Cmdline()
		if err != nil {
			continue
		}
		if re.Match([]byte(c)) {
			pFound = append(pFound, p)
		}
	}
	return pFound, nil

}

func findProcessByCmd(re *regexp.Regexp, ps []*process.Process) []*process.Process {
	pFound := make([]*process.Process, 0)
	for _, p := range ps {
		c, err := p.Cmdline()
		if err != nil {
			continue
		}
		if re.Match([]byte(c)) {
			pFound = append(pFound, p)
		}
	}
	return pFound
}

func assertMetrics(t *testing.T, expectedStr, actual string, ignoredEventAttributes []string) bool {
	var v []map[string]interface{}
	if err := json.Unmarshal([]byte(actual), &v); err != nil {
		t.Error(err)
		t.FailNow()
	}
	for i := range v {
		events := v[i]["Events"].([]interface{})
		for i := range events {
			event := events[i].(map[string]interface{})
			for _, attr := range ignoredEventAttributes {
				delete(event, attr)
			}
		}
	}
	var expected []map[string]interface{}
	assert.Nil(t, json.Unmarshal([]byte(expectedStr), &expected))

	return assert.Equal(t, expected, v)
}

func traceRequests(ch chan http.Request) {
	for {
		select {
		case req := <-ch:
			bodyBuffer, _ := ioutil.ReadAll(req.Body)
			fmt.Println(string(bodyBuffer))
		}
	}
}

func getProcessNameRegExp(name string) *regexp.Regexp {
	expr := fmt.Sprintf(`spawner(.*)-nri-process-name %s(.*)`, name)
	return regexp.MustCompile(expr)
}

func createFile(from, dest string, vars map[string]interface{}) error {
	outputFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	t, err := template.ParseFiles(from)
	if err != nil {
		return err

	}
	return t.Execute(outputFile, vars)
}

func templatePath(filename string) string {
	return filepath.Join("testdata", "templates", filename)
}
