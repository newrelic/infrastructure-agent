// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metric

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/stretchr/testify/require"
)

const TruncLen = 10

type TestCase struct {
	Description string
	Input       sample.Event
	Expected    sample.Event
}

func TestTruncates(t *testing.T) {
	testCases := []TestCase{
		{Description: "Do not Truncate struct",
			Input:    Test{A: "foo", B: 123, C: "bar"},
			Expected: Test{A: "foo", B: 123, C: "bar"}},
		{Description: "Truncate struct",
			Input:    Test{A: "modified field", B: 123, C: "not modify"},
			Expected: Test{A: "modified f", B: 123, C: "not modify"}},
		{Description: "Truncate struct2",
			Input:    Test{A: "not modify", B: 123, C: "modified field"},
			Expected: Test{A: "not modify", B: 123, C: "modified f"}},
		{Description: "Truncate nested structs",
			Input:    Nested{Test: Test{A: "hello", C: "bar"}, C: "modified field", D: "do not"},
			Expected: Nested{Test: Test{A: "hello", C: "bar"}, C: "modified f", D: "do not"}},
		{Description: "Truncate interface map",
			Input:    InterfaceMap{"A": "modified field", "B": 123, "C": "not modify"},
			Expected: InterfaceMap{"A": "modified f", "B": 123, "C": "not modify"}},
		{Description: "Do not limit interface map",
			Input:    InterfaceMap{"A": "foo", "B": 123, "C": "bar"},
			Expected: InterfaceMap{"A": "foo", "B": 123, "C": "bar"}},
	}
	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			limited := TruncateLength(tc.Input, TruncLen)
			require.Equal(t, tc.Expected, limited)
		})
	}
}

func TestTruncates_PassedAsPointers(t *testing.T) {
	type Test struct {
		A string
		B int
		C string
		D float64
		E string
	}

	testCases := []TestCase{
		{Description: "null input and output",
			Input: nil, Expected: nil},
		{Description: "Do not Truncate struct",
			Input:    &PTest{A: "foo", B: 123, C: "bar"},
			Expected: &PTest{A: "foo", B: 123, C: "bar"}},
		{Description: "Truncate struct",
			Input:    &PTest{A: "modified field", B: 123, C: "not modify"},
			Expected: &PTest{A: "modified f", B: 123, C: "not modify"}},
		{Description: "Truncate interface map",
			Input:    &InterfaceMap{"A": "modified field", "B": 123, "C": "not modify"},
			Expected: &InterfaceMap{"A": "modified f", "B": 123, "C": "not modify"}},
		{Description: "Do not limit interface map",
			Input:    &InterfaceMap{"A": "foo", "B": 123, "C": "bar"},
			Expected: &InterfaceMap{"A": "foo", "B": 123, "C": "bar"}},
		{Description: "Truncate string map",
			Input:    &StringMap{"A": "modified field", "C": "not modify"},
			Expected: &StringMap{"A": "modified f", "C": "not modify"}},
		{Description: "Do not limit string map",
			Input:    &StringMap{"A": "foo", "C": "bar"},
			Expected: &StringMap{"A": "foo", "C": "bar"}},
	}
	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			limited := TruncateLength(tc.Input, TruncLen)
			require.Equal(t, tc.Expected, limited)
		})
	}
}

func TestTruncates_Containing_Pointers(t *testing.T) {
	longString := "modified field"
	cutLongString := "modified f"
	shortString := "ThisIsFine"
	anyInt := 456

	// looking for edge cases
	var pShortString interface{} = &shortString
	var pAnyInt interface{} = &anyInt

	testCases := []TestCase{
		{Description: "Do not Truncate struct",
			Input:    TestP{A: &shortString, B: &anyInt, C: &shortString},
			Expected: TestP{A: &shortString, B: &anyInt, C: &shortString}},
		{Description: "Truncate struct",
			Input:    TestP{A: &longString, B: &anyInt, C: &shortString},
			Expected: TestP{A: &cutLongString, B: &anyInt, C: &shortString}},
		{Description: "Truncate interface map",
			Input:    InterfaceMap{"A": &longString, "B": &anyInt, "C": &shortString, "D": nil},
			Expected: InterfaceMap{"A": &cutLongString, "B": &anyInt, "C": &shortString, "D": nil}},
		{Description: "Do not limit interface map",
			Input:    InterfaceMap{"A": &shortString, "B": &anyInt, "C": &shortString, "D": nil},
			Expected: InterfaceMap{"A": &shortString, "B": &anyInt, "C": &shortString, "D": nil}},
		{Description: "Do not limit interface pointer map",
			Input:    InterfacePointerMap{"A": &pShortString, "B": &pAnyInt, "C": &pShortString, "D": nil},
			Expected: InterfacePointerMap{"A": &pShortString, "B": &pAnyInt, "C": &pShortString, "D": nil}},
		{Description: "Do not limit string pointer map",
			Input:    StringPointerMap{"A": &shortString, "C": &shortString, "D": nil},
			Expected: StringPointerMap{"A": &shortString, "C": &shortString, "D": nil}},
	}
	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			limited := TruncateLength(tc.Input, TruncLen)
			require.Equal(t, tc.Expected, limited)
		})
	}

}

func TestTruncates_Containing_Pointers_Map_Modify(t *testing.T) {
	longString := "modified field"
	cutLongString := "modified f"
	shortString := "ThisIsFine"
	anyInt := 456

	// looking for edge cases
	var pShortString interface{} = &shortString
	var pAnyInt interface{} = &anyInt
	var pLongString interface{} = &longString
	var pCutLongString interface{} = &cutLongString
	testCases := []TestCase{
		{Description: "Truncate interface pointer map",
			Input:    InterfacePointerMap{"A": &pLongString, "B": &pAnyInt, "C": &pShortString, "D": nil},
			Expected: InterfacePointerMap{"A": &pCutLongString, "B": &pAnyInt, "C": &pShortString, "D": nil}},
		{Description: "Truncate string pointer map",
			Input:    StringPointerMap{"A": &longString, "C": &shortString, "D": nil},
			Expected: StringPointerMap{"A": &cutLongString, "C": &shortString, "D": nil}},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			limited := TruncateLength(tc.Input, TruncLen)
			switch l := limited.(type) {
			case InterfacePointerMap:
				ex := tc.Expected.(InterfacePointerMap)
				require.Equal(t, ex["A"], l["A"])
				require.Equal(t, ex["B"], l["B"])
				require.Equal(t, ex["C"], l["C"])
				require.Equal(t, ex["D"], l["D"])
			case StringPointerMap:
				ex := tc.Expected.(StringPointerMap)
				require.Equal(t, ex["A"], l["A"])
				require.Equal(t, ex["B"], l["B"])
				require.Equal(t, ex["C"], l["C"])
				require.Equal(t, ex["D"], l["D"])
			default:
				require.Failf(t, "Unexpected return type", "%T", limited)
			}
		})
	}
}

func BenchmarkTruncateLength_Struct(b *testing.B) {

}

type Nested struct {
	Test Test
	C    string
	D    string
}

func (t Nested) Entity(_ entity.Key) {}
func (t Nested) Type(_ string)       {}
func (t Nested) Timestamp(_ int64)   {}

type Test struct {
	A string
	B int
	C string
	D float64
	E string
}

func (t Test) Entity(_ entity.Key) {}
func (t Test) Type(_ string)       {}
func (t Test) Timestamp(_ int64)   {}

type PTest struct {
	A string
	B int
	C string
	D float64
	E string
}

func (t *PTest) Entity(_ entity.Key) {}
func (t *PTest) Type(_ string)       {}
func (t *PTest) Timestamp(_ int64)   {}

type TestP struct {
	A *string
	B *int
	C *string
	D *float64
	E *string
}

func (t TestP) Type(_ string)       {}
func (t TestP) Entity(_ entity.Key) {}
func (t TestP) Timestamp(_ int64)   {}

type InterfaceMap map[string]interface{}

func (t InterfaceMap) Entity(_ entity.Key) {}
func (t InterfaceMap) Type(_ string)       {}
func (t InterfaceMap) Timestamp(_ int64)   {}

type StringMap map[string]string

func (t StringMap) Entity(_ entity.Key) {}
func (t StringMap) Type(_ string)       {}
func (t StringMap) Timestamp(_ int64)   {}

type InterfacePointerMap map[string]*interface{}

func (t InterfacePointerMap) Entity(_ entity.Key) {}
func (t InterfacePointerMap) Type(_ string)       {}
func (t InterfacePointerMap) Timestamp(_ int64)   {}

type StringPointerMap map[string]*string

func (t StringPointerMap) Entity(_ entity.Key) {}
func (t StringPointerMap) Type(_ string)       {}
func (t StringPointerMap) Timestamp(_ int64)   {}
