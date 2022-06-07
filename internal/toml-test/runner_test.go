package tomltest

import (
	"os"
	"testing"
)

func notInList(t *testing.T, list []string, str string) {
	t.Helper()
	for _, item := range list {
		if item == str {
			t.Fatalf("error: %q in list", str)
		}
	}
}

func TestVersion(t *testing.T) {
	_, err := Runner{Version: "0.9", Files: os.DirFS("./tests")}.Run()
	if err == nil {
		t.Fatal("expected an error for version 0.9")
	}

	r := Runner{Version: "1.0.0", Files: os.DirFS("./tests")}
	ls, err := r.List()
	if err != nil {
		t.Fatal()
	}
	notInList(t, ls, "valid/string/escape-esc")

	r = Runner{Version: "0.4.0", Files: os.DirFS("./tests")}
	ls, err = r.List()
	if err != nil {
		t.Fatal()
	}
	notInList(t, ls, "valid/string/escape-esc")      // 1.0
	notInList(t, ls, "valid/array/mixed-int-string") // 0.5
	notInList(t, ls, "valid/key/dotted")             // 0.4
}
