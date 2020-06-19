// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package log

import "io"

type MemLogger struct {
	buf    []byte
	writer io.Writer
}

func NewMemLogger(writer io.Writer) *MemLogger {
	return &MemLogger{
		writer: writer,
	}
}

func (mem *MemLogger) Write(b []byte) (n int, err error) {
	mem.buf = append(mem.buf, b...)
	return mem.writer.Write(b)
}

func (mem *MemLogger) WriteBuffer(writer io.Writer) (n int, err error) {
	return writer.Write(mem.buf)
}
