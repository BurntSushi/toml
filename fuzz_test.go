package toml_test

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	toml "github.com/BurntSushi/toml"
)

//go:embed _example/*.toml
//go:embed internal/toml-test/tests/invalid/array/*.toml
//go:embed internal/toml-test/tests/invalid/bool/*.toml
//go:embed internal/toml-test/tests/invalid/control/*.toml
//go:embed internal/toml-test/tests/invalid/datetime/*.toml
//go:embed internal/toml-test/tests/invalid/encoding/*.toml
//go:embed internal/toml-test/tests/invalid/float/*.toml
//go:embed internal/toml-test/tests/invalid/inline-table/*.toml
//go:embed internal/toml-test/tests/invalid/integer/*.toml
//go:embed internal/toml-test/tests/invalid/key/*.toml
//go:embed internal/toml-test/tests/invalid/local-date/*.toml
//go:embed internal/toml-test/tests/invalid/local-datetime/*.toml
//go:embed internal/toml-test/tests/invalid/local-time/*.toml
//go:embed internal/toml-test/tests/invalid/spec-1.0.0/*.toml
//go:embed internal/toml-test/tests/invalid/spec-1.1.0/*.toml
//go:embed internal/toml-test/tests/invalid/string/*.toml
//go:embed internal/toml-test/tests/invalid/table/*.toml
//go:embed internal/toml-test/tests/valid/*.toml
//go:embed internal/toml-test/tests/valid/array/*.toml
//go:embed internal/toml-test/tests/valid/bool/*.toml
//go:embed internal/toml-test/tests/valid/comment/*.toml
//go:embed internal/toml-test/tests/valid/datetime/*.toml
//go:embed internal/toml-test/tests/valid/float/*.toml
//go:embed internal/toml-test/tests/valid/inline-table/*.toml
//go:embed internal/toml-test/tests/valid/integer/*.toml
//go:embed internal/toml-test/tests/valid/key/*.toml
//go:embed internal/toml-test/tests/valid/spec-1.0.0/*.toml
//go:embed internal/toml-test/tests/valid/spec-1.1.0/*.toml
//go:embed internal/toml-test/tests/valid/string/*.toml
//go:embed internal/toml-test/tests/valid/table/*.toml
//go:embed testdata/*.toml
var fuzzCorpus embed.FS

func FuzzDecode(f *testing.F) {
	err := fs.WalkDir(fuzzCorpus, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		input, err := fuzzCorpus.ReadFile(path)
		if err != nil {
			return err
		}
		f.Add(string(input))
		return nil
	})
	if err != nil {
		f.Fatalf("walk fuzz corpus: %v", err)
	}

	f.Add("\"\n\"")
	f.Add(`"`)
	f.Add(`"\r\n`)
	f.Add(string("\"\\"))
	f.Add(string("\"\n"))
	f.Add("\"\r\n\"")
	f.Add("\"\r\n")
	f.Add("\n")
	f.Add(`
# This is an example TOML document which shows most of its features.

# Simple key/value with a string.
title = "TOML example \U0001F60A"

desc = """
An example TOML document. \
"""

# Array with integers and floats in the various allowed formats.
integers = [42, 0x42, 0o42, 0b0110]
floats   = [1.42, 1e-02]

# Array with supported datetime formats.
times = [
	2021-11-09T15:16:17+01:00,  # datetime with timezone.
	2021-11-09T15:16:17Z,       # UTC datetime.
	2021-11-09T15:16:17,        # local datetime.
	2021-11-09,                 # local date.
	15:16:17,                   # local time.
]

# Durations.
duration = ["4m49s", "8m03s", "1231h15m55s"]

# Table with inline tables.
distros = [
	{name = "Arch Linux", packages = "pacman"},
	{name = "Void Linux", packages = "xbps"},
	{name = "Debian",     packages = "apt"},
]

# Create new table; note the "servers" table is created implicitly.
[servers.alpha]
	# You can indent as you please, tabs or spaces.
	ip        = '10.0.0.1'
	hostname  = 'server1'
	enabled   = false
[servers.beta]
	ip        = '10.0.0.2'
	hostname  = 'server2'
	enabled   = true

# Start a new table array; note that the "characters" table is created implicitly.
[[characters.star-trek]]
	name = "James Kirk"
	rank = "Captain\u0012 \t"
[[characters.star-trek]]
	name = "Spock"
	rank = "Science officer"

[undecoded] # To show the MetaData.Undecoded() feature.
	key = "This table intentionally left undecoded"
`)
	var before, after map[string]any
	buf := bytes.NewBuffer(make([]byte, 0, 1024*16))
	f.Fuzz(func(t *testing.T, file string) {
		t.Log(file)
		clear(before)
		clear(after)
		buf.Reset()
		_, err := toml.Decode(file, &before)
		if err != nil {
			t.Run("no panic in ParseError.ErrorWithPosition", func(t *testing.T) {
				defer func() {
					if p := recover(); p != nil {
						stack := debug.Stack()
						t.Errorf("panic: %v\n%s", p, stack)
					}
				}()

				// Make sure ErrorWithPosition doesn't panic.
				var pErr toml.ParseError
				if errors.As(err, &pErr) {
					t.Logf("file=%q, err=%s", file, err.Error())
					pErr.ErrorWithPosition()
				}

			})
			return

		}
		clear(after)

		buf.Reset()
		toml.NewEncoder(buf).Encode(before)

		_, err = toml.Decode(buf.String(), &after)
		if err != nil {
			t.Fatalf("error decoding encoded TOML: %v", err)
			return
		}
		t.Run("no go-level diff", func(t *testing.T) {

			for _, d := range Diff(before, after) {
				t.Error(d)
			}

		})
		t.Run("encoding is stable", func(t *testing.T) {
			encoded := buf.String()
			buf.Reset()
			toml.NewEncoder(buf).Encode(after)
			encoded2 := buf.String()
			if encoded != encoded2 {
				t.Errorf("encoding is not stable:\nfirst:\n%s\nsecond:\n%s", encoded, encoded2)
			}
		})

	})
}

func TestDiff(t *testing.T) {

	before := map[string]any{
		"a": int64(1),
		"b": "string",
		"c": true,
		"d": []map[string]any{
			{"x": int64(1)},
		},
		"e": map[string]any{
			"y": "nested",
		},
		"f": []any{int64(1), "two", false},
		"g": float64(1.23),
	}

	for _, tt := range []struct {
		name  string
		after map[string]any
		keys  []string
	}{
		{name: "no diffs", after: before},
		{
			name: "type differences",
			after: map[string]any{
				"a": []any{int64(1)},
				"b": int64(2),
				"c": struct{ b bool }{true},
				"d": []map[string]any{
					{"x": "one"},
				},
				"e": map[string]any{
					"y": int64(1),
				},
				"f": []any{float64(1.1), 2, []any{true}},
				"g": "1.23",
			},
			keys: []string{"a", "b", "c", "d[0].x", "e.y", "f[0]", "f[1]", "f[2]", "g"},
		},
		{name: "value differences", after: map[string]any{
			"a": int64(1),
			"b": "string",
			"c": false,
			"d": []map[string]any{
				{"x": int64(2)},
			},
			"e": map[string]any{
				"y": "changed",
			},
			"f": []any{int64(1), "two", true},
			"g": float64(4.56),
		}, keys: []string{"c", "d[0].x", "e.y", "f[2]", "g"}},
	} {
		t.Run(tt.name+"/forward", func(t *testing.T) { checkDiff(t, before, tt.after, tt.keys) })
		t.Run(tt.name+"/backward", func(t *testing.T) { checkDiff(t, tt.after, before, tt.keys) })

	}
}
func checkDiff(t testing.TB, a, b any, keys []string) {
	t.Helper()
	diffs := Diff(a, b)
	if len(diffs) == 0 && len(keys) > 0 {
		t.Errorf("expected diffs, got none")
	}
	if len(diffs) != len(keys) {
		for _, d := range diffs {
			t.Log(d)
		}
	FINDDIFF:
		for _, k := range keys {
			for _, d := range diffs {
				if strings.Contains(d, k) {
					continue FINDDIFF
				}
			}
			t.Errorf("%s: missing diff", k)
		}

	}
}

// compare two elements and return a slice of differences as strings.
// this compares recursively element-by-element a-la `reflect.DeepEqual` with the following differences:
// - Any NaN is equal to any other NaN
// - map[string][]T is type-erased to map[string][]any before comparison (ATTENTION: is this the right behavior?)
func Diff(a, b any) []string { return appendElemDiff(nil, a, b, make([]byte, 0, 512)) }

// appendMapDiffs compares two map[string]any and appends any differences to the diffs slice.
func appendMapDiffs(diffs []string, a, b map[string]any, prefix []byte) []string {
	if len(a) != len(b) {
		return append(diffs, fmt.Sprintf("%s (%T): want len=%d, got len=%d", string(prefix), a, len(a), len(b)))
	}
	for k, v0 := range a {
		diffs = appendElemDiff(diffs, v0, b[k], append(append(prefix, '.'), k...))
	}
	return diffs
}

// appendListDiff compares two []any and appends any differences to the diffs slice.
func appendListDiff(diffs []string, a, b []any, prefix []byte) []string {
	if len(a) != len(b) {
		return append(diffs, fmt.Sprintf("%s (%T): len(a)=%d, len(b)=%d", string(prefix), a, len(a), len(b)))
	}
	for i, v0 := range a {
		v1 := b[i]
		diffs = appendElemDiff(diffs, v0, v1, fmt.Appendf(prefix, "[%d]", i))
	}
	return diffs
}

// appendValDiff compares two values of type T and appends any differences to the diffs slice.
func appendValDiff[T comparable](diffs []string, a T, b any, prefix []byte) []string {
	if bt, ok := b.(T); !ok || a != bt {
		return append(diffs, fmt.Sprintf("%s: want %#+v, got %#+v", prefix, a, b))
	}
	return diffs
}

// typeErase turns a []T into a []any.
func typeErase[T any](a []T) []any {
	erased := make([]any, len(a))
	for i, v := range a {
		erased[i] = v
	}
	return erased
}

// appendElemDiff compares two elements v0 and v1 and appends any differences to the diffs slice.
func appendElemDiff(diffs []string, v0, v1 any, prefix []byte) []string {
	switch v0 := v0.(type) {
	case string:
		return appendValDiff(diffs, v0, v1, prefix)
	case int64:
		return appendValDiff(diffs, v0, v1, prefix)
	case bool:
		return appendValDiff(diffs, v0, v1, prefix)
	case []any:
		// special case: v1 might be a []map[string]any
		if spec, ok := v1.([]map[string]any); ok {
			return appendListDiff(diffs, v0, typeErase(spec), prefix)
		}
		v1l, ok := v1.([]any)
		if !ok {
			return append(diffs, fmt.Sprintf("%s:want a %#+v, got a %#+v", prefix, v0, v1))
		}
		return appendListDiff(diffs, v0, v1l, prefix)
	case []map[string]any:
		return appendElemDiff(diffs, typeErase(v0), v1, prefix)
	case map[string]any:
		v1m, ok := v1.(map[string]any)
		if !ok {
			return append(diffs, fmt.Sprintf("%s: want a %T, got a %T", prefix, v0, v1))
		}
		return appendMapDiffs(diffs, v0, v1m, prefix)

	case float64:
		if v1f, ok := v1.(float64); !ok || v0 != v1f && !(math.IsNaN(v0) && math.IsNaN(v1f)) {
			return append(diffs, fmt.Sprintf("%s: want %#+v, got %#+v", prefix, v0, v1))
		}
		return nil
	case time.Time:
		if v1t, ok := v1.(time.Time); !ok || !v0.Equal(v1t) {
			return append(diffs, fmt.Sprintf("%s: want %#+v, got %#+v", prefix, v0, v1))
		}
		return diffs
	case time.Duration:
		return appendValDiff(diffs, v0, v1, prefix)
	case nil:
		if v1 != nil {
			return append(diffs, fmt.Sprintf("%s: want nil, got %#+v", prefix, v1))
		}
		return diffs
	default:
		return append(diffs, fmt.Sprintf("%s: unsupported type %T", prefix, v0))
	}
}
