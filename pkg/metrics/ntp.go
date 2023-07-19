/*
 *
 *  * Copyright 2021 New Relic Corporation. All rights reserved.
 *  * SPDX-License-Identifier: Apache-2.0
 *
 */

package metrics

import (
	"errors"
	"time"

	"github.com/beevik/ntp"
	"go.uber.org/multierr"
)

const (
	ntpIntervalMin    = 15 // minutes
	ntpTimeoutDefault = 5  // seconds
)

var (
	ErrEmptyNtpHosts    = errors.New("ntp host list is empty")
	ErrGettingNtpOffset = errors.New("cannot get ntp offset")
	ErrNotInInterval    = errors.New("cannot query ntp servers offset inside interval")
)

type Ntp struct {
	pool      []string
	timeout   time.Duration // ntp request timeout in seconds
	interval  time.Duration // ntp request interval in minutes
	updatedAt time.Time     // last time the ntp offset was fetched
	now       func() time.Time
	ntpQuery  func(host string, opt ntp.QueryOptions) (*ntp.Response, error)
}

// NewNtp creates a new Ntp instance
// timeout is expressed in secods
// interval is expressed in minutes.
func NewNtp(pool []string, timeout uint, interval uint) *Ntp {
	validInterval := guardInterval(interval)
	validTimeout := guardTimeout(timeout)
	return &Ntp{
		pool:     pool,
		timeout:  time.Second * time.Duration(validTimeout),
		interval: time.Minute * time.Duration(validInterval),
		now:      time.Now,
		ntpQuery: ntp.QueryWithOptions,
	}
}

// guardTimeout ensures that interval is not 0
// when interval is 0 the used library defaults to 5s, so we check it here
// to decouple from that decision in case that it changes.
func guardTimeout(timeout uint) uint {
	if timeout == 0 {
		return ntpTimeoutDefault
	}
	return timeout
}

// guardInterval ensures that interval is not smaller than ntpIntervalMin
// if it is not valid, it will default to ntpIntervalMin.
func guardInterval(interval uint) uint {
	if interval < ntpIntervalMin {
		return ntpIntervalMin
	}
	return interval
}

// Offset returns the Ntp servers offset.
func (p *Ntp) Offset() (time.Duration, error) {
	if len(p.pool) == 0 {
		return 0, ErrEmptyNtpHosts
	}

	if p.now().Sub(p.updatedAt) < p.interval {
		// return error in case outside query interval
		return 0, ErrNotInInterval
	}

	// update current interval even if error
	defer func() {
		p.updatedAt = p.now()
	}()

	var offsets []time.Duration

	var ntpQueryErr error
	for _, host := range p.pool {
		response, err := p.ntpQuery(host, ntp.QueryOptions{Timeout: p.timeout})
		if err != nil {
			ntpQueryErr = multierr.Append(ntpQueryErr, err)
			syslog.WithError(err).WithField("ntp_host", host).WithField("timeout", p.timeout).Debug("error getting ntp offset")

			continue
		}

		err = response.Validate()
		if err != nil {
			ntpQueryErr = multierr.Append(ntpQueryErr, err)
			syslog.WithError(err).WithField("ntp_host", host).WithField("timeout", p.timeout).Debug("error validating ntp response, skipping")

			continue
		}

		syslog.WithField("ntp_host", host).WithField("response", response).Trace("valid ntp response retrieved")
		offsets = append(offsets, response.ClockOffset)
	}

	if len(offsets) == 0 {
		return 0, multierr.Append(ErrGettingNtpOffset, ntpQueryErr)
	}

	// calculate average from all hosts values
	var total time.Duration
	for _, offset := range offsets {
		total += offset
	}

	return total / time.Duration(len(offsets)), nil
}
