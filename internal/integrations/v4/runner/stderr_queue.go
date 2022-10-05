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
	defaultStderrQueueLen = 10
)

type stderrQueue struct {
	mutex    sync.Mutex
	nextLine int
	size     int
	queue    [][]byte
}

func newStderrQueue(size int) stderrQueue {
	maxSize := size
	if maxSize == 0 {
		maxSize = defaultStderrQueueLen
	} else if maxSize < 0 {
		maxSize = 0
	}

	return stderrQueue{
		queue: make([][]byte, maxSize),
		size:  maxSize,
	}
}

func (sq *stderrQueue) Add(line []byte) {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()

	if sq.size == 0 {
		return
	}

	sq.queue[sq.nextLine%sq.size] = line
	sq.nextLine++
}

func (sq *stderrQueue) Flush() string {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()
	if sq.nextLine == 0 || sq.size == 0 {
		return "(no standard error output)"
	}
	var start, lines int
	joint := bytes.Buffer{}
	if sq.nextLine < sq.size {
		start = 0
		lines = sq.nextLine
	} else {
		joint.WriteString(fmt.Sprintf("(last %d lines out of %d): ", sq.size, sq.nextLine))
		start = sq.nextLine % sq.size
		lines = sq.size
	}
	for i := 0; i < lines; i++ {
		if i > 0 {
			joint.WriteByte('\n')
		}
		joint.Write(sq.queue[start])
		start = (start + 1) % sq.size
	}
	sq.nextLine = 0
	return joint.String()
}
