// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	WRITABLE_MASK = 0222
	READABLE_MASK = 0444
)

var (
	sclog               = log.WithPlugin("Sysctl")
	ignoredListPatterns = []string{
		`kernel/((ns_last_pid)|(msgmni)|(sched_domain/cpu\d+?/domain\d+?/max_newidle_lb_cost))`,
		`net/(.+)/(((conf|neigh)/veth([\w\d]+?)/)|(base_reachable_time$))`,
		`fs/((protected_hardlinks)|(protected_symlinks))`,
		`kernel/(cad_pid)`,
		`kernel/usermodehelper/((bset)|(inheritable))`,
		`net/core/((bpf_jit_harden)|(bpf_jit_kallsyms))`,
		`net/ipv4/(tcp_fastopen_key)`,
		`net/ipv6/conf/(.+)/(stable_secret)`,
		`vm/((mmap_rnd_bits)|(mmap_rnd_compat_bits)|(stat_refresh))`,
	}
)

type walker func(root string, walkFn filepath.WalkFunc) error
type reader func(filename string) ([]byte, error)

// fileService abstracts the common file operations
type fileService struct {
	walk walker
	read reader
}

type SysctlItem struct {
	Sysctl string `json:"id"`
	Value  string `json:"sysctl_value"`
}

func (self SysctlItem) SortKey() string {
	return self.Sysctl
}
