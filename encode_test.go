package toml

import (
	"bytes"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := map[string]struct {
		input      interface{}
		wantOutput string
	}{
		"bool field": {
			input: struct {
				BoolTrue  bool
				BoolFalse bool
			}{true, false},
			wantOutput: "BoolTrue = true\nBoolFalse = false",
		},
		"int fields": {
			input: struct {
				Int   int
				Int8  int8
				Int16 int16
				Int32 int32
				Int64 int64
			}{1, 2, 3, 4, 5},
			wantOutput: "Int = 1\nInt8 = 2\nInt16 = 3\nInt32 = 4\nInt64 = 5",
		},
		"uint fields": {
			input: struct {
				Uint   uint
				Uint8  uint8
				Uint16 uint16
				Uint32 uint32
				Uint64 uint64
			}{1, 2, 3, 4, 5},
			wantOutput: "Uint = 1\nUint8 = 2\nUint16 = 3\nUint32 = 4\nUint64 = 5",
		},
		"float fields": {
			input: struct {
				Float32 float32
				Float64 float64
			}{1.5, 2.5},
			wantOutput: "Float32 = 1.5\nFloat64 = 2.5",
		},
		"string field": {
			input:      struct{ String string }{"foo"},
			wantOutput: `String = "foo"`,
		},
	}
	for label, test := range tests {
		var buf bytes.Buffer
		e := newEncoder(&buf)
		if err := e.Encode(test.input); err != nil {
			t.Errorf("%s: Encode failed: %s", label, err)
			continue
		}
		test.wantOutput += "\n"
		if got := buf.String(); test.wantOutput != got {
			t.Errorf("%s: want %q, got %q", label, test.wantOutput, got)
		}
	}
}
