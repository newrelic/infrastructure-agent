// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package executor implements the executor module which is able to execute an individual command and
// forward the standard input, standard error, and go errors by a set of channels
package executor

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/gobackfill"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/constants"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const unknownErrExitCode = -3

var illog = log.WithComponent("integrations.Executor")

// Executor handles Executable commands asynchronously.
type Executor struct {
	Cfg     *Config
	Command string
	Args    []string
}

// FromCmdSlice builds a Executor instance from a string slices, being the first element
// the command path/name and the next, the command arguments
func FromCmdSlice(cmd []string, cfg *Config) Executor {
	return Executor{Cfg: cfg, Command: cmd[0], Args: cmd[1:]}
}

// Execute runs the command in background, sending by a channel the standard output and error, as well as any execution
// error may happen (task can't start, task is killed...).
// The executed process can be cancelled via the provided Context.
// When writable PID channel is provided, generated PID will be written, so process could be signaled by 3rd parties.
// When the process ends, all the channels are closed.
func (r *Executor) Execute(ctx context.Context, pidChan, exitCodeCh chan<- int) OutputReceive {
	out, receiver := NewOutput()
	commandCtx, cancelCommand := context.WithCancel(ctx)

	go func() {
		cmd := r.buildCommand(commandCtx)

		illog.
			WithField("command", r.Command).
			WithField("path", cmd.Path).
			// TODO: creates weird failure on leaktest
			//WithField("args", helpers.ObfuscateSensitiveDataFromArray(cmd.Args)).
			WithField("env", helpers.ObfuscateSensitiveDataFromArray(cmd.Env)).
			Debug("Running command.")

		// redirecting stdin and stdout for on-the-go scanning
		cmdOutput, err := cmd.StdoutPipe()
		if err != nil {
			out.Errors <- err
			return
		}
		cmdError, err := cmd.StderrPipe()
		if err != nil {
			out.Errors <- err
			return
		}

		// allows closing OutputSend only after the task is finished and all the data is read
		allOutputForwarded := sync.WaitGroup{}
		allOutputForwarded.Add(2)

		// scans standard output and error pipes and forwards individual lines to a channel
		go func() {
			defer allOutputForwarded.Done()
			forwardCmdOutput(cmdOutput, out.Stdout, out.Errors)
		}()
		go func() {
			defer allOutputForwarded.Done()
			forwardCmdOutput(cmdError, out.Stderr, out.Errors)
		}()

		// on normal output, when the output pipes are closed, we cancel the
		// command context so the parent goroutine can exit
		go func() {
			allOutputForwarded.Wait()
			cancelCommand()
		}()

		if err := cmd.Start(); err != nil {
			out.Errors <- err
		}

		if pidChan != nil {
			pidChan <- cmd.Process.Pid
		}

		// Waits for the command to finish (or be externally cancelled) and closes
		// the OutputSend channels when all the data has been submitted
		<-commandCtx.Done()
		if err := cmd.Wait(); err != nil {
			out.Errors <- err
			if exitCodeCh != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCodeCh <- gobackfill.ExitCode(exitError)
				} else {
					exitCodeCh <- unknownErrExitCode
				}
			}
		} else if exitCodeCh != nil {
			exitCodeCh <- 0
		}

		allOutputForwarded.Wait() // waiting again to avoid closing output before the data is received during cancellation
		out.Close()
	}()
	return receiver
}

// reads lines from stdout or stderr and forwards them to the fwd channel
func forwardCmdOutput(buffer io.Reader, fwd chan<- []byte, errors chan<- error) {
	lineReader := bufio.NewReader(buffer)

	// reads a line from stoud/stderr
	line, err := lineReader.ReadBytes('\n')
	for err == nil {
		// removes trailing new line symbols to only forward the json payload
		line = bytes.TrimRight(line, "\r\n")
		fwd <- line
		line, err = lineReader.ReadBytes('\n')
	}
	if err != io.EOF {
		errors <- err
	}
	// if, after EOF, there was any data in the buffer, it submits it as a new line
	if len(line) > 0 {
		fwd <- line
	}
}

func (r *Executor) buildCommand(ctx context.Context) *exec.Cmd {
	cmd := r.userAwareCmd(ctx)
	cmd.Env = os.Environ()
	for key, val := range r.Cfg.BuildEnv() {
		cmd.Env = append(cmd.Env, key+"="+val)
	}

	enableVerbose, ok := ctx.Value(constants.EnableVerbose).(int)

	if ok && enableVerbose > 0 {
		cmd.Env = append(cmd.Env, "VERBOSE=1")
	}

	cmd.Dir = r.Cfg.Directory
	return cmd
}

// DeepClone returns an exact copy of an Executor, without references to the same data structures.
// It will allow replacing ${config.path} variables by the agent in several executor instances.
func (r *Executor) DeepClone() Executor {
	argsCopy := make([]string, len(r.Args))
	copy(argsCopy, r.Args)

	return Executor{
		Cfg:     r.Cfg.deepClone(),
		Command: r.Command, // as strings are immutable we don't need to clone it
		Args:    argsCopy,
	}
}
