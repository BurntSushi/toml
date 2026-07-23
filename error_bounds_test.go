package toml_test

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestErrorWithPositionOutOfRangeDoesNotPanic(t *testing.T) {
	var v map[string]any
	err := toml.Unmarshal([]byte("key = "), &v)
	if err == nil {
		t.Fatal("expected error")
	}
	pErr, ok := err.(toml.ParseError)
	if !ok {
		t.Fatalf("type %T", err)
	}
	// Corrupt position to the single-line EOF class of values from #498.
	pErr.Position = toml.Position{Line: 0, Col: 0, Len: 0}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	s := pErr.ErrorWithPosition()
	if s == "" {
		t.Fatal("empty ErrorWithPosition")
	}
	pErr.Position = toml.Position{Line: 999, Col: 1, Len: 1}
	_ = pErr.ErrorWithPosition()
}
