package toml

import (
	"errors"
	"runtime/debug"
	"testing"
)

func TestErrorWithPositionCRLF(t *testing.T) {
	for _, input := range []string{
		"\"\n\"",
		"\"",
		"\"\r\n\"",
		"\"\r\n",
		"\"\\",
		"\"\n",
		"\"\n\"",
	} {
		var m map[string]any
		_, err := Decode(input, &m)
		if err == nil {
			t.Fatalf("expected error, got nil for %q", input)
		}
		var pErr ParseError
		if !errors.As(err, &pErr) {
			t.Fatalf("expected ParseError for %q, got %T: %v", input, err, err)
		}
		if pErr.Position.Line < 1 {
			t.Errorf("Position.Line=%d for %q; want >= 1", pErr.Position.Line, input)
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic for %q: %v\n%s", input, r, debug.Stack())
				}
			}()
			_ = pErr.ErrorWithPosition()
		}()
	}
}
