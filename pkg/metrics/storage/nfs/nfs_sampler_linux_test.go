// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package nfs

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
)

func Test_compareNFSOps(t *testing.T) {
	last := []procfs.NFSOperationStats{
		{
			Operation: "READ",
			Requests:  10,
		},
		{
			Operation: "WRITE",
			Requests:  5,
		},
		{
			Operation: "SOMETHING_ELSE",
			Requests:  5,
		},
	}
	current := []procfs.NFSOperationStats{
		{
			Operation: "READ",
			Requests:  30,
		},
		{
			Operation: "WRITE",
			Requests:  15,
		},
		{
			Operation: "SOMETHING_ELSE",
			Requests:  15,
		},
	}
	lastRun := time.Now()
	total, reads, writes := compareNFSOps(last, current, lastRun, lastRun.Add(time.Second*10))

	if total != float64(4) {
		t.Errorf("compareNFSOps() total = %f, want %f", total, float64(4))
	}
	if reads != float64(2) {
		t.Errorf("compareNFSOps() reads = %f, want %f", reads, float64(2))
	}
	if writes != float64(1) {
		t.Errorf("compareNFSOps() writes = %f, want %f", writes, float64(1))
	}
}

func Test_parseNFSOps(t *testing.T) {
	ops := []procfs.NFSOperationStats{
		{
			Operation: "READ",
			Requests:  20,
		},
		{
			Operation: "WRITE",
			Requests:  10,
		},
		{
			Operation: "SOMETHING_ELSE",
			Requests:  10,
		},
	}
	total, reads, writes := parseNFSOps(ops)

	if total != 40 {
		t.Errorf("parseNFSOps() total = %d, want %d", total, 40)
	}
	if reads != 20 {
		t.Errorf("parseNFSOps() reads = %d, want %d", reads, 20)
	}
	if writes != 10 {
		t.Errorf("parseNFSOps() writes = %d, want %d", writes, 10)
	}
}

func Test_nfsStatDelta(t *testing.T) {
	type args struct {
		last      uint64
		current   uint64
		lastRun   time.Time
		checkTime time.Time
	}

	now := time.Now()
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "validDelta",
			args: args{
				last:      uint64(100),
				current:   uint64(150),
				lastRun:   now,
				checkTime: now.Add(time.Second * 10),
			},
			want: float64(5),
		},
		{
			name: "isNaN",
			args: args{
				last:      uint64(150),
				current:   uint64(150),
				lastRun:   now,
				checkTime: now,
			},
			want: float64(0),
		},
		{
			name: "isInf",
			args: args{
				last:      uint64(100),
				current:   uint64(150),
				lastRun:   now,
				checkTime: now,
			},
			want: float64(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nfsStatDelta(tt.args.last, tt.args.current, tt.args.lastRun, tt.args.checkTime); got != tt.want {
				t.Errorf("nfsStatDelta() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseFloat(t *testing.T) {
	ptr := func(f float64) *float64 {
		return &f
	}
	type args struct {
		f float64
	}
	tests := []struct {
		name string
		args args
		want *float64
	}{
		{
			name: "whenNaN",
			args: args{f: math.NaN()},
			want: nil,
		},
		{
			name: "whenNotNaN",
			args: args{f: 76.0},
			want: ptr(76.0),
		},
		{
			name: "when-Inf",
			args: args{f: math.Inf(1)},
			want: nil,
		},
		{
			name: "when+Inf",
			args: args{f: math.Inf(0)},
		},
		{
			name: "whenNotInf",
			args: args{f: 76.0},
			want: ptr(76.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, parseFloat(tt.args.f), tt.want)
		})
	}
}
