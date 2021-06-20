package toml_test

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	tomltest "github.com/BurntSushi/toml-test"
)

/*
func TestDecodeError(t *testing.T) {
	file :=
		`a = "a"
b = "b"
c = 001  # invalid
`

	var s struct {
		A, B string
		C    int
	}
	_, err := Decode(file, &s)
	if err == nil {
		t.Fatal("err is nil")
	}

	var dErr DecodeError
	if !errors.As(err, &dErr) {
		t.Fatalf("err is not a DecodeError: %T %[1]v", err)
	}

	want := DecodeError{
		Line:    3,
		Pos:     17,
		LastKey: "c",
		Message: `Invalid integer "001": cannot have leading zeroes`,
	}
	if !reflect.DeepEqual(dErr, want) {
		t.Errorf("unexpected data\nhave: %#v\nwant: %#v", dErr, want)
	}

}
*/

func TestParseError(t *testing.T) {
	fsys := tomltest.EmbeddedTests()
	ls, err := fs.ReadDir(fsys, "invalid")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range ls {
		if !strings.HasSuffix(f.Name(), ".toml") {
			continue
		}

		if f.Name() == "string-multiline-escape-space.toml" {
			continue
		}

		input, err := fs.ReadFile(fsys, "invalid/"+f.Name())
		if err != nil {
			t.Fatal(err)
		}

		var x interface{}
		_, err = toml.Decode(string(input), &x)
		if err == nil {
			continue
		}

		var dErr toml.ParseError
		if !errors.As(err, &dErr) {
			t.Errorf("err is not a ParseError: %T %[1]v", err)
			continue
		}

		fmt.Println()
		fmt.Println("–––", f.Name(), strings.Repeat("–", 65-len(f.Name())))
		fmt.Print(dErr.ExtError())
		fmt.Println(strings.Repeat("–", 70))
	}
}
