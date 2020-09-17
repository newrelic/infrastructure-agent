// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestParseFBOutput(t *testing.T) {
	//
	// 2020/03/09 20:21:54 [DEBUG] Error making HTTP request.  Got status code: 403
	tests := []struct {
		name         string
		line         string
		wantSaneLine string
		wantSev      logrus.Level
	}{
		{
			name:         "fb-info is converted to agent-debug",
			line:         "[2020/03/04 15:57:54] [ info] [input] pausing tail.0",
			wantSaneLine: "[input] pausing tail.0",
			wantSev:      logrus.DebugLevel,
		},
		{
			name:         "warning",
			line:         "[2020/03/04 15:57:54] [ warn] some warning",
			wantSaneLine: "some warning",
			wantSev:      logrus.WarnLevel,
		},
		{
			name:         "error",
			line:         "[2020/03/16 17:14:01] [error] [sqldb] error=database is locked",
			wantSaneLine: "[sqldb] error=database is locked",
			wantSev:      logrus.ErrorLevel,
		},
		{
			name:         "debug",
			line:         "2020/03/10 07:08:21 [DEBUG] Error making HTTP request: Post https://log-api.newrelic.com/log/v1: x509: certificate has expired or is not yet valid",
			wantSaneLine: "Error making HTTP request: Post https://log-api.newrelic.com/log/v1: x509: certificate has expired or is not yet valid",
			wantSev:      logrus.DebugLevel,
		},
		{
			name:         "skip undesired headers",
			line:         "Fluent Bit v1.3.9",
			wantSaneLine: "",
			wantSev:      logrus.DebugLevel,
		},
		{
			name:         "keep unknown long lines",
			line:         "aaa bbb ccc ddd eee fff ggg hhh iii jjj kkk lll mmm nnn rrr sss",
			wantSaneLine: "aaa bbb ccc ddd eee fff ggg hhh iii jjj kkk lll mmm nnn rrr sss",
			wantSev:      logrus.DebugLevel,
		},
		{
			name:         "drop colored stuff targeted to foreground visualization",
			line:         "\x1b[1m\x1b[93m foo bar",
			wantSaneLine: "",
			wantSev:      logrus.DebugLevel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSanitizedLine, gotSeverity := ParseFBOutput(tt.line)
			assert.Equal(t, tt.wantSaneLine, gotSanitizedLine)
			assert.Equal(t, tt.wantSev.String(), gotSeverity.String())
		})
	}
}
