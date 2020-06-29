// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"fmt"
	"github.com/google/shlex"
	"time"
)

type Command struct {
	Exec        ShlexOpt          `yaml:"exec"`
	Environment map[string]string `yaml:"env"`
	Matcher     map[string]string `yaml:"match"`
	Timeout     time.Duration     `yaml:"timeout"`
}

func (c *Command) Validate() error {
	if len(c.Exec) == 0 {
		return errors.New("missing 'cmd' entries")
	}
	if len(c.Matcher) == 0 {
		return errors.New("missing 'match' entries")
	}
	return nil
}

// ShlexOpt is a wrapper around []string so we can use go-shlex for shell tokenizing
type ShlexOpt []string

// Set the value
func (s *ShlexOpt) Set(value string) error {
	valueSlice, err := shlex.Split(value)
	*s = valueSlice
	return err
}

// Type returns the type of the value
func (s *ShlexOpt) Type() string {
	return "command"
}

func (s *ShlexOpt) String() string {
	if len(*s) == 0 {
		return ""
	}
	return fmt.Sprint(*s)
}

func (s *ShlexOpt) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		return s.Set(single)
	} else {
		*s = multi
	}
	return nil
}

// Value returns the value as a string slice
func (s *ShlexOpt) Value() []string {
	return *s
}
