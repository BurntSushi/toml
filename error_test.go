//go:build go1.16
// +build go1.16

package toml_test

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	tomltest "github.com/BurntSushi/toml/internal/toml-test"
)

func TestErrorPosition(t *testing.T) {
	// Note: take care to use leading spaces (not tabs).
	tests := []struct {
		test, err string
	}{
		{"array/missing-separator.toml", `
toml: error: expected a comma (',') or array terminator (']'), but got '2'

At line 1, column 13:

      1 | wrong = [ 1 2 3 ]
                      ^`},

		{"array/no-close-2.toml", `
toml: error: expected a comma (',') or array terminator (']'), but got end of file

At line 1, column 10:

      1 | x = [42 #
                   ^`},

		{"array/tables-2.toml", `
toml: error: Key 'fruit.variety' has already been defined.

At line 9, column 3-8:

      7 | 
      8 |   # This table conflicts with the previous table
      9 |   [fruit.variety]
             ^^^^^`},
		{"datetime/trailing-t.toml", `
toml: error: Invalid TOML Datetime: "2006-01-30T".

At line 2, column 4-15:

      1 | # Date cannot end with trailing T
      2 | d = 2006-01-30T
              ^^^^^^^^^^^`},
	}

	fsys := tomltest.EmbeddedTests()
	for _, tt := range tests {
		t.Run(tt.test, func(t *testing.T) {
			input, err := fs.ReadFile(fsys, "invalid/"+tt.test)
			if err != nil {
				t.Fatal(err)
			}

			var x interface{}
			_, err = toml.Decode(string(input), &x)
			if err == nil {
				t.Fatal("err is nil")
			}

			var pErr toml.ParseError
			if !errors.As(err, &pErr) {
				t.Errorf("err is not a ParseError: %T %[1]v", err)
			}

			tt.err = tt.err[1:] + "\n" // Remove first newline, and add trailing.
			want := pErr.ErrorWithUsage()

			if !strings.Contains(want, tt.err) {
				t.Fatalf("\nwant:\n%s\nhave:\n%s", tt.err, want)
			}
		})
	}
}

// Useful to print all errors, to see if they look alright.
func TestParseError(t *testing.T) {
	return // Doesn't need to be part of the test suite.

	fsys := tomltest.EmbeddedTests()
	err := fs.WalkDir(fsys, ".", func(path string, f fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".toml") {
			return nil
		}

		if f.Name() == "string-multiline-escape-space.toml" || f.Name() == "bad-utf8-at-end.toml" {
			return nil
		}

		input, err := fs.ReadFile(fsys, path)
		if err != nil {
			t.Fatal(err)
		}

		var x interface{}
		_, err = toml.Decode(string(input), &x)
		if err == nil {
			return nil
		}

		var pErr toml.ParseError
		if !errors.As(err, &pErr) {
			t.Errorf("err is not a ParseError: %T %[1]v", err)
			return nil
		}

		fmt.Println()
		fmt.Println("\x1b[1m━━━", path, strings.Repeat("━", 65-len(path)), "\x1b[0m")
		fmt.Print(pErr.Error())
		fmt.Println()
		fmt.Println("─── ErrorWithLocation()", strings.Repeat("–", 47))
		fmt.Print(pErr.ErrorWithPosition())
		fmt.Println("─── ErrorWithUsage()", strings.Repeat("–", 50))
		fmt.Print(pErr.ErrorWithUsage())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
