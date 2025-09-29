//go:build windows
// +build windows

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package winapi

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUTF16PtrToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Simple ASCII string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "String with spaces",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "String with numbers",
			input:    "test123",
			expected: "test123",
		},
		{
			name:     "String with special characters",
			input:    "test@#$%",
			expected: "test@#$%",
		},
		{
			name:     "Long string",
			input:    "this is a much longer string to test the function with more data",
			expected: "this is a much longer string to test the function with more data",
		},
		{
			name:     "String with unicode characters",
			input:    "héllo wørld",
			expected: "héllo wørld",
		},
		{
			name:     "Windows path-like string",
			input:    "\\Processor(_Total)\\% Processor Time",
			expected: "\\Processor(_Total)\\% Processor Time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Convert input string to UTF16 pointer using syscall
			utf16Ptr, err := syscall.UTF16PtrFromString(tt.input)
			require.NoError(t, err, "Failed to create UTF16 pointer from string")

			// Test the function
			result := UTF16PtrToString(utf16Ptr)

			// Verify the result
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUTF16PtrToString_NilPointer(t *testing.T) {
	t.Parallel()

	// Test with nil pointer
	result := UTF16PtrToString(nil)
	assert.Equal(t, "", result, "Should return empty string for nil pointer")
}

func TestUTF16PtrToString_ZeroLengthString(t *testing.T) {
	t.Parallel()

	// Create a UTF16 string that points to a null terminator (zero-length string)
	nullTerminator := uint16(0)
	ptr := &nullTerminator

	result := UTF16PtrToString(ptr)
	assert.Equal(t, "", result, "Should return empty string for zero-length UTF16 string")
}

func TestUTF16PtrToString_ManualUTF16Array(t *testing.T) {
	t.Parallel()

	// Manually create a UTF16 array: "test" + null terminator
	utf16Array := []uint16{
		0x0074, // 't'
		0x0065, // 'e'
		0x0073, // 's'
		0x0074, // 't'
		0x0000, // null terminator
	}

	// Get pointer to first element
	ptr := &utf16Array[0]

	result := UTF16PtrToString(ptr)
	assert.Equal(t, "test", result, "Should correctly parse manually created UTF16 array")
}

func TestUTF16PtrToString_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Single character", func(t *testing.T) {
		t.Parallel()

		utf16Ptr, err := syscall.UTF16PtrFromString("A")
		require.NoError(t, err)

		result := UTF16PtrToString(utf16Ptr)
		assert.Equal(t, "A", result)
	})

	t.Run("String with newlines", func(t *testing.T) {
		t.Parallel()

		input := "line1\nline2\r\nline3"
		utf16Ptr, err := syscall.UTF16PtrFromString(input)
		require.NoError(t, err)

		result := UTF16PtrToString(utf16Ptr)
		assert.Equal(t, input, result)
	})

	t.Run("String with tabs", func(t *testing.T) {
		t.Parallel()

		input := "col1\tcol2\tcol3"
		utf16Ptr, err := syscall.UTF16PtrFromString(input)
		require.NoError(t, err)

		result := UTF16PtrToString(utf16Ptr)
		assert.Equal(t, input, result)
	})
}

// BenchmarkUTF16PtrToString benchmarks the performance of the function.
func BenchmarkUTF16PtrToString(b *testing.B) {
	// Create test string
	testString := "\\Processor(_Total)\\% Processor Time"
	utf16Ptr, err := syscall.UTF16PtrFromString(testString)
	require.NoError(b, err)

	b.ResetTimer()

	for range b.N {
		_ = UTF16PtrToString(utf16Ptr)
	}
}

// BenchmarkUTF16PtrToString_LongString benchmarks with a longer string.
func BenchmarkUTF16PtrToString_LongString(b *testing.B) {
	// Create a longer test string
	testString := "This is a much longer string to test the performance of the UTF16PtrToString function " +
		"with more realistic data that might be encountered in real-world scenarios"
	utf16Ptr, err := syscall.UTF16PtrFromString(testString)
	require.NoError(b, err)

	b.ResetTimer()

	for range b.N {
		_ = UTF16PtrToString(utf16Ptr)
	}
}

// TestUTF16PtrToString_CompareWithSyscall verifies our implementation matches syscall behavior.
func TestUTF16PtrToString_CompareWithSyscall(t *testing.T) {
	t.Parallel()

	testStrings := []string{
		"hello",
		"hello world",
		"\\Processor(_Total)\\% Processor Time",
		"test with unicode: héllo wørld 你好",
		"",
		"A",
		"special chars: !@#$%^&*()",
	}

	for _, testStr := range testStrings {
		t.Run("Compare_"+testStr, func(t *testing.T) {
			t.Parallel()

			// Convert to UTF16 slice first, then get pointer
			utf16Slice, err := syscall.UTF16FromString(testStr)
			require.NoError(t, err)

			// Get pointer to first element (if slice is not empty)
			if len(utf16Slice) > 0 {
				ptr := &utf16Slice[0]

				// Test our function
				ourResult := UTF16PtrToString(ptr)

				// Compare with syscall's implementation
				syscallResult := syscall.UTF16ToString(utf16Slice[:len(utf16Slice)-1]) // Remove null terminator for syscall

				assert.Equal(t, syscallResult, ourResult, "Our implementation should match syscall.UTF16ToString")
			}
		})
	}
}
