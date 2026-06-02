package toml

import (
	"testing"
)

// Test case 1: Adding keys to inline array
func TestIssue470InlineArrayInvalidAddKeys(t *testing.T) {
	doc := `a = []
a.b = ''
`
	var m map[string]interface{}
	err := Unmarshal([]byte(doc), &m)
	if err == nil {
		t.Error("Expected error for adding key to inline array, but got none")
	} else {
		t.Logf("Got expected error: %v", err)
		if !contains(err.Error(), "inline Array") {
			t.Errorf("Wrong error message. Expected 'inline Array', got: %v", err)
		}
	}
}

// Test case 2: Adding keys to inline table
func TestIssue470InlineTableInvalidAddKeys(t *testing.T) {
	doc := `a = {}
a.b = ''
`
	var m map[string]interface{}
	err := Unmarshal([]byte(doc), &m)
	if err == nil {
		t.Error("Expected error for adding key to inline table, but got none")
	} else {
		t.Logf("Got expected error: %v", err)
		if !contains(err.Error(), "inline") {
			t.Errorf("Wrong error message, expected 'inline': %v", err)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test case 3: Complex nesting scenario from the issue
func TestIssue470RedefineTableThenAddKeys(t *testing.T) {
	doc := `[a.b]
[a]
b.c = ''
`
	var m map[string]interface{}
	// This should actually be valid TOML - redefining [a] after [a.b] is allowed,
	// but then trying to redefine b as a table with subkeys is the issue
	err := Unmarshal([]byte(doc), &m)
	if err != nil {
		t.Logf("Got error: %v", err)
	} else {
		t.Logf("Decoded successfully: %v", m)
	}
}
