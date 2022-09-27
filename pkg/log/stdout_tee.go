// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"io"
	"os"
)

// stdoutTeeLogger is a simple logging wrapper which copies all output to both stdout and a log file to make it easier to find.
// This is nice for Windows, since there's nothing built-in to capture all stdout from a program into some
// kind of syslog, and we don't want to flood the system event log with uninteresting messages.
type stdoutTeeLogger struct {
	writer io.Writer
	stdout bool
}

// NewStdoutTeeLogger return an io.Writer that can redirect to stdout if needed.
func NewStdoutTeeLogger(writer io.Writer, stdout bool) io.Writer {
	return &stdoutTeeLogger{
		writer: writer,
		stdout: stdout,
	}
}

func (s *stdoutTeeLogger) Write(b []byte) (n int, err error) {
	if s.stdout {
		_, _ = os.Stdout.Write(b)
	}
	return s.writer.Write(b)
}
