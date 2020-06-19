// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"bytes"
	"fmt"
	"sync"
)

// allows printing the latest error lines if the integration exited prematurely
const (
	stderrQueueLen = 10
)

type stderrQueue struct {
	mutex    sync.Mutex
	nextLine int
	queue    [stderrQueueLen][]byte
}

func (sq *stderrQueue) Add(line []byte) {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()
	sq.queue[sq.nextLine%stderrQueueLen] = line
	sq.nextLine++
}

func (sq *stderrQueue) Flush() string {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()
	if sq.nextLine == 0 {
		return "(no standard error output)"
	}
	var start, lines int
	joint := bytes.Buffer{}
	if sq.nextLine < stderrQueueLen {
		start = 0
		lines = sq.nextLine
	} else {
		joint.WriteString(fmt.Sprintf("(last %d lines out of %d): ", stderrQueueLen, sq.nextLine))
		start = sq.nextLine % stderrQueueLen
		lines = stderrQueueLen
	}
	for i := 0; i < lines; i++ {
		if i > 0 {
			joint.WriteByte('\n')
		}
		joint.Write(sq.queue[start])
		start = (start + 1) % stderrQueueLen
	}
	sq.nextLine = 0
	return joint.String()
}
