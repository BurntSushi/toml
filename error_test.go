// +build go1.16

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

func TestParseError(t *testing.T) {
	return
	fsys := tomltest.EmbeddedTests()
	ls, err := fs.ReadDir(fsys, "invalid")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range ls {
		if !strings.HasSuffix(f.Name(), ".toml") {
			continue
		}
		if f.Name() != "datetime-no-secs.toml" {
			//continue
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
		fmt.Println("━━━", f.Name(), strings.Repeat("━", 65-len(f.Name())))
		fmt.Print(dErr.Error())
		fmt.Println()
		fmt.Println(strings.Repeat("–", 70))
		fmt.Print(dErr.ExtendedWithUsage())
		fmt.Println(strings.Repeat("━", 70))
	}
}
