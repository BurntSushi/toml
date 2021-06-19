package toml_test

import (
	"testing"

	tomltest "github.com/BurntSushi/toml-test"
)

func TestToml(t *testing.T) {
	run := func(t *testing.T, enc bool) {
		t.Helper()
		r := tomltest.Runner{
			Files:     tomltest.EmbeddedTests(),
			Encoder:   enc,
			ParserCmd: []string{"toml-test-decoder"},
			SkipTests: []string{"valid/key-dotted",
				"valid/datetime-local-date",
				"valid/datetime-local-time",
				"valid/datetime-local"},
		}
		if enc {
			r.ParserCmd = []string{"toml-test-encoder"}
			r.SkipTests = append(r.SkipTests, "valid/inline-table-nest")
		}

		tests, err := r.Run()
		if err != nil {
			t.Fatal(err)
		}

		for _, test := range tests.Tests {
			if test.Failed() {
				t.Run(test.Path, func(t *testing.T) {
					t.Errorf("\nError:\n%s\n\nInput:\n%s\nOutput:\n%s\nWant:\n%s\n",
						test.Failure, test.Input, test.Output, test.Want)
				})
			}
		}
		t.Logf("passed: %d; failed: %d; skipped: %d", tests.Passed, tests.Failed, tests.Skipped)
	}

	t.Run("decode", func(t *testing.T) { run(t, false) })
	t.Run("encode", func(t *testing.T) { run(t, true) })
}
