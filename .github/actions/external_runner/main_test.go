// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"regexp"
	"testing"
)

var (
	logAnsibleFilters = []string{"TASK\\s\\[.*\\]\\s", "PLAY\\s\\[.*\\]\\s", "PLAY\\sRECAP\\s", "ok=\\d+\\s+changed=\\d+\\s+unreachable=\\d+\\s+failed=\\d+\\s+skipped=\\d+\\s+rescued=\\d+\\s+ignored=\\d+"}
)

func TestPrintLogLine(t *testing.T) {
	var tests = []struct {
		name          string
		regexps       []string
		inputLines    []string
		expectedLines string
	}{
		{"no regexp defined", nil, []string{"test"}, ""},
		{"match everything regexp", []string{".*"}, []string{"test number two"}, "test number two\n"},
		{"ansible regexp", logAnsibleFilters, []string{"test"}, ""},
		{"ansible regexp with task output",
			logAnsibleFilters,
			[]string{
				"EVENTS  123   TASK [cleanup : stop newrelic-infra service] ***********************************        123",
				"EVENTS  123   changed: [amd64:ubuntu20.10]    123",
				"EVENTS  123   included: /srv/newrelic/infrastructure-agent/test/packaging/ansible/roles/cleanup/tasks/package-Debian.yaml for amd64:ubuntu20.10, amd64:ubuntu18.04, amd64:ubuntu16.04, amd64:debian-buster, amd64:debian-bullseye, amd64:ubuntu21.04, amd64:debian-stretch, amd64:ubuntu20.04, amd64:ubuntu22.04, arm64:ubuntu21.04, arm64:debian-bullseye, arm64:ubuntu22.04, arm64:ubuntu18.04, arm64:ubuntu20.10, arm64:debian-buster, arm64:debian-stretch, arm64:ubuntu20.04, arm64:ubuntu16.04  123",
				"EVENTS  123   PLAY [log-forwarder-amd64] *****************************************************        123",
			},
			"EVENTS  123   TASK [cleanup : stop newrelic-infra service] ***********************************        123\nEVENTS  123   PLAY [log-forwarder-amd64] *****************************************************        123\n"},
		{"ansible regexp with recap output",
			logAnsibleFilters,
			[]string{
				"EVENTS  123   PLAY RECAP *********************************************************************        123",
				"EVENTS  123   amd64:al-2                 : ok=307  changed=78   unreachable=0    failed=0    skipped=36   rescued=0    ignored=0      123",
				"EVENTS  123   amd64:al-2022              : ok=48   changed=12   unreachable=0    failed=1    skipped=6    rescued=0    ignored=0      123",
				"EVENTS  123   amd64:centos-stream        : ok=326  changed=89   unreachable=0    failed=0    skipped=35   rescued=0    ignored=0      123",
			},
			"EVENTS  123   PLAY RECAP *********************************************************************        123\nEVENTS  123   amd64:al-2                 : ok=307  changed=78   unreachable=0    failed=0    skipped=36   rescued=0    ignored=0      123\nEVENTS  123   amd64:al-2022              : ok=48   changed=12   unreachable=0    failed=1    skipped=6    rescued=0    ignored=0      123\nEVENTS  123   amd64:centos-stream        : ok=326  changed=89   unreachable=0    failed=0    skipped=35   rescued=0    ignored=0      123\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logFilters []*regexp.Regexp
			for _, filter := range tt.regexps {
				logFilters = append(logFilters, regexp.MustCompile(filter))
			}

			actualOutput := bytes.NewBufferString("")
			for _, line := range tt.inputLines {
				printLogLine(line, actualOutput, logFilters)
			}

			if actualOutput.String() != tt.expectedLines {
				t.Errorf("got %q, want %q", actualOutput, tt.expectedLines)
			}
		})
	}
}
