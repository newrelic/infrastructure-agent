// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package metrics

import "testing"

func TestRunWithUnavailablePrivilege(t *testing.T) {
	err := RunWithPrivilege("SeCreateTokenPrivilege", func() error { return nil })
	if _, ok := err.(*PrivilegeError); err == nil || !ok {
		t.Fatal("expected PrivilegeError")
	}
}

func TestRunWithPrivileges(t *testing.T) {
	err := RunWithPrivilege("SeShutdownPrivilege", func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
}

// Requires Admin to run
// func TestRunWithDebug(t *testing.T) {
// 	err := RunWithPrivilege("SeDebugPrivilege", func() error { return nil })
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }
