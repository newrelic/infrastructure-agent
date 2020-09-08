// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"math"
	"strconv"
	"testing"
)

func TestAttributesWriteJSON(t *testing.T) {
	tests := []struct {
		key    string
		val    interface{}
		expect string
	}{
		{"string", "string", `{"string":"string"}`},
		{"true", true, `{"true":true}`},
		{"false", false, `{"false":false}`},
		{"uint8", uint8(1), `{"uint8":1}`},
		{"uint16", uint16(1), `{"uint16":1}`},
		{"uint32", uint32(1), `{"uint32":1}`},
		{"uint64", uint64(1), `{"uint64":1}`},
		{"uint", uint(1), `{"uint":1}`},
		{"uintptr", uintptr(1), `{"uintptr":1}`},
		{"int8", int8(1), `{"int8":1}`},
		{"int16", int16(1), `{"int16":1}`},
		{"int32", int32(1), `{"int32":1}`},
		{"int64", int64(1), `{"int64":1}`},
		{"int", int(1), `{"int":1}`},
		{"float32", float32(1), `{"float32":1}`},
		{"float64", float64(1), `{"float64":1}`},
		{"default", func() {}, `{"default":"func()"}`},
		{"NaN", math.NaN(), `{"NaN":"NaN"}`},
		{"positive-infinity", math.Inf(1), `{"positive-infinity":"infinity"}`},
		{"negative-infinity", math.Inf(-1), `{"negative-infinity":"infinity"}`},
	}

	for _, test := range tests {
		buf := &bytes.Buffer{}
		ats := Attributes(map[string]interface{}{
			test.key: test.val,
		})
		ats.WriteJSON(buf)
		got := string(buf.Bytes())
		if got != test.expect {
			t.Errorf("key='%s' val=%v expect='%s' got='%s'",
				test.key, test.val, test.expect, got)
		}
	}
}

func TestEmptyAttributesWriteJSON(t *testing.T) {
	var ats Attributes
	buf := &bytes.Buffer{}
	ats.WriteJSON(buf)
	got := string(buf.Bytes())
	if got != `{}` {
		t.Error(got)
	}
}

func TestOrderedAttributesWriteJSON(t *testing.T) {
	ats := map[string]interface{}{
		"z": 123,
		"b": "hello",
		"a": true,
		"x": 13579,
		"m": "zap",
		"c": "zip",
	}
	got := string(MarshalOrderedAttributes(ats))
	if got != `{"a":true,"b":"hello","c":"zip","m":"zap","x":13579,"z":123}` {
		t.Error(got)
	}
}

func TestEmptyOrderedAttributesWriteJSON(t *testing.T) {
	got := string(MarshalOrderedAttributes(nil))
	if got != `{}` {
		t.Error(got)
	}
}

func sampleAttributes(num int) map[string]interface{} {
	attributes := make(map[string]interface{})
	for i := 0; i < num; i++ {
		istr := strconv.Itoa(i)
		// Mix string and integer attributes:
		if i%2 == 0 {
			attributes[istr] = istr
		} else {
			attributes[istr] = i
		}
	}
	return attributes
}

func BenchmarkAttributes(b *testing.B) {
	attributes := Attributes(sampleAttributes(1000))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Compare this to: `js, err := json.Marshal(attributes)`
		buf := &bytes.Buffer{}
		attributes.WriteJSON(buf)
		if 0 == buf.Len() {
			b.Fatal(buf.Len())
		}
	}
}

func BenchmarkOrderedAttributes(b *testing.B) {
	attributes := sampleAttributes(1000)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Compare this to: `js, err := json.Marshal(attributes)`
		js := MarshalOrderedAttributes(attributes)
		if len(js) == 0 {
			b.Fatal(string(js))
		}
	}
}
