// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ipc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_String(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		expected string
	}{
		{
			name:     "custom message",
			msg:      Message("custom"),
			expected: "custom",
		},
		{
			name:     "empty message",
			msg:      Message(""),
			expected: "",
		},
		{
			name:     "message with spaces",
			msg:      Message("hello world"),
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.msg))
		})
	}
}

func TestMessage_TypeConversion(t *testing.T) {
	// Test that Message can be used as a string
	msg := Message("test")
	s := string(msg)
	assert.Equal(t, "test", s)

	// Test that string can be converted to Message
	str := "another test"
	m := Message(str)
	assert.Equal(t, Message("another test"), m)
}

func TestMessage_Equality(t *testing.T) {
	msg1 := Message("test")
	msg2 := Message("test")
	msg3 := Message("different")

	assert.Equal(t, msg1, msg2)
	assert.NotEqual(t, msg1, msg3)
}

func TestMessage_EmptyMessage(t *testing.T) {
	var empty Message
	assert.Equal(t, Message(""), empty)
	assert.Empty(t, string(empty))
}

func TestMessage_MapKey(t *testing.T) {
	// Test that Message can be used as a map key
	handlers := make(map[Message]func() error)

	handlers[Message("test")] = func() error { return nil }
	handlers[Message("another")] = func() error { return nil }

	assert.Len(t, handlers, 2)
	assert.NotNil(t, handlers[Message("test")])
	assert.NotNil(t, handlers[Message("another")])
	assert.Nil(t, handlers[Message("nonexistent")])
}
