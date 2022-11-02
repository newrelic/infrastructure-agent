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
)

type Ntp struct {
	pool      []string
	timeout   time.Duration // ntp request timeout in seconds
	interval  time.Duration // ntp request interval in minutes
	updatedAt time.Time     // last time the ntp offset was fetched
	offset    time.Duration // cache for last offset value retrieved
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

func (p *Ntp) Offset() (time.Duration, error) {
	if len(p.pool) == 0 {
		return 0, ErrEmptyNtpHosts
	}

	// We only query the servers once every p.interval
	if p.now().Sub(p.updatedAt) < p.interval {
		return p.offset, nil
	}

	var offsets []time.Duration

	var ntpQueryErr error
	for _, host := range p.pool {
		response, err := p.ntpQuery(host, ntp.QueryOptions{Timeout: p.timeout})
		if err == nil {
			offsets = append(offsets, response.ClockOffset)
		} else {
			ntpQueryErr = multierr.Append(ntpQueryErr, err)
			syslog.WithError(err).WithField("ntp_host", host).WithField("timeout", p.timeout).Debug("error getting ntp offset")
		}
	}

	if len(offsets) == 0 {
		return 0, multierr.Append(ErrGettingNtpOffset, ntpQueryErr)
	}

	// calculate average from all hosts values
	var total time.Duration
	for _, offset := range offsets {
		total += offset
	}

	// cache the value to be reused
	p.offset = total / time.Duration(len(offsets))
	p.updatedAt = p.now()

	return p.offset, nil
}
