package toml

import (
	"testing"
)

// 测试：ErrorWithPosition 在特殊行号时的数组越界问题
func TestErrorWithPositionOutOfBounds(t *testing.T) {
	tests := []struct {
		name  string
		input string
		line  int
		col   int
		len   int
	}{
		{
			name:  "line 1 with line>1 check but only 1 line total",
			input: "key = ",
			line:  1,
			col:   6,
			len:   1,
		},
		{
			name:  "line number exceeds total lines",
			input: "key = value",
			line:  5,  // 但实际只有 1 行
			col:   1,
			len:   1,
		},
		{
			name:  "line 0 or negative",
			input: "key = value",
			line:  0,
			col:   1,
			len:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := ParseError{
				Position: Position{
					Line: tt.line,
					Col:  tt.col,
					Len:  tt.len,
				},
				Message: "test error",
				input:   tt.input,
			}

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ErrorWithPosition panic: %v", r)
				}
			}()

			result := pe.ErrorWithPosition()
			t.Logf("Result:\n%s", result)
		})
	}
}
