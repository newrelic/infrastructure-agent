// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package executor

// OutputSend holds information about a running task
type OutputSend struct {
	// Stdout is received line by line. It is closed when the task ends
	Stdout chan<- []byte
	// Stderr is received line by line. It is closed when the task ends
	Stderr chan<- []byte
	// Errors receives any execution error or error exit status. It is closed when the task ends
	Errors chan<- error
	// Done is a channel that is closed when the integration has finished
	Done chan<- struct{}
}

// OutputReceive is a receive-only view of OutputSend, made for the sake of safety.
type OutputReceive struct {
	// Stdout is received line by line. It is closed when the task ends
	Stdout <-chan []byte
	// Stderr is received line by line. It is closed when the task ends
	Stderr <-chan []byte
	// Errors receives any execution error or error exit status. It is closed when the task ends
	Errors <-chan error
	// Done is a channel that is closed when the integration has finished
	Done <-chan struct{}
}

// NewOutput creates a default OutputSend group as well as the read-only view.
func NewOutput() (OutputSend, OutputReceive) {
	// For a perfect synchronization between the task executor and the output reader,
	// the OutputSend and its receiver have to be created at the same time.
	const channelsCapacity = 10
	sout := make(chan []byte, channelsCapacity)
	serr := make(chan []byte, channelsCapacity)
	errs := make(chan error, channelsCapacity)
	done := make(chan struct{})
	return OutputSend{
			Stdout: sout,
			Stderr: serr,
			Errors: errs,
			Done:   done,
		},
		OutputReceive{
			Stdout: sout,
			Stderr: serr,
			Errors: errs,
			Done:   done,
		}
}

// Close closes all the channels of a task output
func (t *OutputSend) Close() {
	close(t.Stdout)
	close(t.Stderr)
	close(t.Errors)
	close(t.Done)
}
