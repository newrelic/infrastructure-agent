// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows
// +build windows

// modified from https://github.com/Microsoft/go-winio
package metrics

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

//sys adjustTokenPrivileges(token windows.Token, releaseAll bool, input *byte, outputSize uint32, output *byte, requiredSize *uint32) (success bool, err error) [true] = advapi32.AdjustTokenPrivileges
//sys impersonateSelf(level uint32) (err error) = advapi32.ImpersonateSelf
//sys revertToSelf() (err error) = advapi32.RevertToSelf
//sys openThreadToken(thread syscall.Handle, accessMask uint32, openAsSelf bool, token *windows.Token) (err error) = advapi32.OpenThreadToken
//sys getCurrentThread() (h syscall.Handle) = GetCurrentThread
//sys lookupPrivilegeValue(systemName string, name string, luid *uint64) (err error) = advapi32.LookupPrivilegeValueW
//sys lookupPrivilegeName(systemName string, luid *uint64, buffer *uint16, size *uint32) (err error) = advapi32.LookupPrivilegeNameW
//sys lookupPrivilegeDisplayName(systemName string, name *uint16, buffer *uint16, size *uint32, languageId *uint32) (err error) = advapi32.LookupPrivilegeDisplayNameW

const (
	SE_PRIVILEGE_ENABLED = 2

	ERROR_NOT_ALL_ASSIGNED syscall.Errno = 1300

	SeBackupPrivilege  = "SeBackupPrivilege"
	SeRestorePrivilege = "SeRestorePrivilege"
	SeDebugPrivilege   = "SeDebugPrivilege"
)

const (
	securityAnonymous = iota
	securityIdentification
	securityImpersonation
	securityDelegation
)

var (
	privNames     = make(map[string]uint64)
	privNameMutex sync.Mutex
)

// PrivilegeError represents an error enabling privileges.
type PrivilegeError struct {
	privileges []uint64
}

func (e *PrivilegeError) Error() string {
	s := ""
	if len(e.privileges) > 1 {
		s = "Could not enable privileges "
	} else {
		s = "Could not enable privilege "
	}
	for i, p := range e.privileges {
		if i != 0 {
			s += ", "
		}
		s += `"`
		s += getPrivilegeName(p)
		s += `"`
	}
	return s
}

// RunWithPrivilege enables a single privilege for a function call.
func RunWithPrivilege(name string, fn func() error) error {
	return RunWithPrivileges([]string{name}, fn)
}

// RunWithPrivileges enables privileges for a function call.
func RunWithPrivileges(names []string, fn func() error) error {
	privileges, err := mapPrivileges(names)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	token, err := newThreadToken()
	if err != nil {
		return err
	}
	defer releaseThreadToken(token)
	err = adjustPrivileges(token, privileges, SE_PRIVILEGE_ENABLED)
	if err != nil {
		return err
	}
	return fn()
}

func mapPrivileges(names []string) ([]uint64, error) {
	var privileges []uint64
	privNameMutex.Lock()
	defer privNameMutex.Unlock()
	for _, name := range names {
		p, ok := privNames[name]
		if !ok {
			err := lookupPrivilegeValue("", name, &p)
			if err != nil {
				return nil, err
			}
			privNames[name] = p
		}
		privileges = append(privileges, p)
	}
	return privileges, nil
}

// EnableProcessPrivileges enables privileges globally for the process.
func EnableProcessPrivileges(names []string) error {
	return enableDisableProcessPrivilege(names, SE_PRIVILEGE_ENABLED)
}

// DisableProcessPrivileges disables privileges globally for the process.
func DisableProcessPrivileges(names []string) error {
	return enableDisableProcessPrivilege(names, 0)
}

func enableDisableProcessPrivilege(names []string, action uint32) error {
	privileges, err := mapPrivileges(names)
	if err != nil {
		return err
	}

	p, _ := windows.GetCurrentProcess()
	var token windows.Token
	err = windows.OpenProcessToken(p, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		return err
	}

	defer token.Close()
	return adjustPrivileges(token, privileges, action)
}

func adjustPrivileges(token windows.Token, privileges []uint64, action uint32) error {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(len(privileges)))
	for _, p := range privileges {
		binary.Write(&b, binary.LittleEndian, p)
		binary.Write(&b, binary.LittleEndian, action)
	}
	prevState := make([]byte, b.Len())
	reqSize := uint32(0)
	success, err := adjustTokenPrivileges(token, false, &b.Bytes()[0], uint32(len(prevState)), &prevState[0], &reqSize)
	if !success {
		return err
	}
	if err == ERROR_NOT_ALL_ASSIGNED {
		return &PrivilegeError{privileges}
	}
	return nil
}

func getPrivilegeName(luid uint64) string {
	var nameBuffer [256]uint16
	bufSize := uint32(len(nameBuffer))
	err := lookupPrivilegeName("", &luid, &nameBuffer[0], &bufSize)
	if err != nil {
		return fmt.Sprintf("<unknown privilege %d>", luid)
	}

	var displayNameBuffer [256]uint16
	displayBufSize := uint32(len(displayNameBuffer))
	var langID uint32
	err = lookupPrivilegeDisplayName("", &nameBuffer[0], &displayNameBuffer[0], &displayBufSize, &langID)
	if err != nil {
		return fmt.Sprintf("<unknown privilege %s>", string(utf16.Decode(nameBuffer[:bufSize])))
	}

	return string(utf16.Decode(displayNameBuffer[:displayBufSize]))
}

func newThreadToken() (windows.Token, error) {
	err := impersonateSelf(securityImpersonation)
	if err != nil {
		return 0, err
	}

	var token windows.Token
	err = openThreadToken(getCurrentThread(), syscall.TOKEN_ADJUST_PRIVILEGES|syscall.TOKEN_QUERY, false, &token)
	if err != nil {
		rerr := revertToSelf()
		if rerr != nil {
			panic(rerr)
		}
		return 0, err
	}
	return token, nil
}

func releaseThreadToken(h windows.Token) {
	err := revertToSelf()
	if err != nil {
		panic(err)
	}
	h.Close()
}
