// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import "github.com/sirupsen/logrus"

// ParseFBOutput sanitizes line (stripping FB severity and timestamp) and provides agent severity.
// Agent severity should be either: Debug, Warn or Error. FB-Info maps to agent-Debug.
// FB lines expected to be format:
// [2020/03/04 15:57:54] [ info] [input] pausing tail.0
// 2020/03/09 20:21:54 [DEBUG] Error making HTTP request.  Got status code: 403
func ParseFBOutput(line string) (sanitizedLine string, agentSeverity logrus.Level) {
	agentSeverity = logrus.DebugLevel

	sanitizedLine = stripNonDebugTimestamps(line)
	if len(sanitizedLine) < 6 {
		return
	}

	// Parsing with string maths because they are cheaper than regexes ¯\_(ツ)_/¯
	if sanitizedLine[0] == '[' {
		// Expected well formatted log:
		// [2020/03/04 15:57:54] [ info] [input] pausing tail.0
		fbSeverity := sanitizedLine[1:6]
		if fbSeverity == " info" {
			sanitizedLine = sanitizedLine[8:]
		} else if fbSeverity == " warn" {
			sanitizedLine = sanitizedLine[8:]
			agentSeverity = logrus.WarnLevel
		} else if fbSeverity == "error" {
			sanitizedLine = sanitizedLine[8:]
			agentSeverity = logrus.ErrorLevel
		}
	} else {
		// there might be still a non bracketed timestamp
		// 2020/03/09 20:21:54 [DEBUG] Error making HTTP request...
		if len(sanitizedLine) < 29 || sanitizedLine[:2] == `\x` {
			// FB headers
			sanitizedLine = ""
		} else if sanitizedLine[21:26] == "DEBUG" {
			// debug entries
			sanitizedLine = sanitizedLine[28:]
		}
		// line is kept, there is no known severity level format available
	}
	return
}

// Non debug lines are expected to be prefixed with timestamp, see caller comment.
func stripNonDebugTimestamps(line string) string {
	const tsPrefixLen = 22
	if len(line) > 0 && line[0] == '[' && len(line) > tsPrefixLen {
		return string(line[tsPrefixLen:])
	}
	return line
}
