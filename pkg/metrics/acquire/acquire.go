// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package acquire provides common functionality for the metrics acquisition
package acquire

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	Timeout    = 3 * time.Second
	ErrTimeout = errors.New("Command timed out.")
)

type Invoker interface {
	Command(string, ...string) ([]byte, error)
}

type Invoke struct{}

func (i Invoke) Command(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	return CombinedOutputTimeout(cmd, Timeout)
}

// CombinedOutputTimeout runs the given command with the given timeout and
// returns the combined output of stdout and stderr.
// If the command times out, it attempts to kill the process.
func CombinedOutputTimeout(c *exec.Cmd, timeout time.Duration) ([]byte, error) {
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	if err := c.Start(); err != nil {
		return nil, err
	}
	err := WaitTimeout(c, timeout)
	return b.Bytes(), err
}

// WaitTimeout waits for the given command to finish with a timeout.
// It assumes the command has already been started.
// If the command times out, it attempts to kill the process.
func WaitTimeout(c *exec.Cmd, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	done := make(chan error)
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		timer.Stop()
		return err
	case <-timer.C:
		if err := c.Process.Kill(); err != nil {
			log.Printf("FATAL error killing process: %s", err)
			return err
		}
		// wait for the command to return after killing it
		<-done
		return ErrTimeout
	}
}

func CalculateSafeDelta(current, previous uint64, elapsedSecs float64) float64 {
	if previous >= current || elapsedSecs <= 0 {
		return float64(0)
	}
	return float64(current-previous) / elapsedSecs
}

// ReadLines reads contents from a file and splits them by new lines.
// A convenience wrapper to ReadLinesOffsetN(filename, 0, -1).
func ReadLines(filename string) ([]string, error) {
	return ReadLinesOffsetN(filename, 0, -1)
}

// ReadLines reads contents from file and splits them by new line.
// The offset tells at which line number to start.
// The count determines the number of lines to read (starting from offset):
//   n >= 0: at most n lines
//   n < 0: whole file
func ReadLinesOffsetN(filename string, offset uint, n int) (ret []string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var line string
	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err = r.ReadString('\n')
		// on EOF we also break, but we should have results
		// on other errors, we should not have any result which we check on the "client"
		if err != nil {
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, err
}
