// +build go1.16

package toml_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	tomltest "github.com/BurntSushi/toml-test"
)

// Test if the error message matches what we want for invalid tests. Every slice
// entry is tested with strings.Contains.
//
// Filepaths are glob'd
var errorTests = map[string][]string{
	"encoding-bad-utf8*":            {"invalid UTF-8 byte"},
	"encoding-utf16*":               {"files cannot contain NULL bytes; probably using UTF-16"},
	"string-multiline-escape-space": {`invalid escape: '\ '`},
}

// Test metadata; all keys listed as "keyname: type".
var metaTests = map[string]string{
	// TODO: this probably should have albums as a Hash as well?
	"table-array-implicit": `
			albums.songs: ArrayHash
			albums.songs.name: String
		`,
}

func TestToml(t *testing.T) {
	for k := range errorTests { // Make sure patterns are valid.
		_, err := filepath.Match(k, "")
		if err != nil {
			t.Fatal(err)
		}
	}

	run := func(t *testing.T, enc bool) {
		t.Helper()
		r := tomltest.Runner{
			Files:   tomltest.EmbeddedTests(),
			Encoder: enc,
			Parser:  parser{},
			SkipTests: []string{
				"valid/datetime-local-date",
				"valid/datetime-local-time",
				"valid/datetime-local",
				"valid/array-mix-string-table",
				"valid/inline-table-nest",

				"invalid/string-literal-multiline-quotes",
			},
		}

		tests, err := r.Run()
		if err != nil {
			t.Fatal(err)
		}

		for _, test := range tests.Tests {
			t.Run(test.Path, func(t *testing.T) {
				if test.Failed() {
					t.Fatalf("\nError:\n%s\n\nInput:\n%s\nOutput:\n%s\nWant:\n%s\n",
						test.Failure, test.Input, test.Output, test.Want)
					return
				}

				// Test metadata
				if !enc && test.Type() == tomltest.TypeValid {
					testMeta(t, test)
				}

				// Test error message.
				if test.Type() == tomltest.TypeInvalid {
					testError(t, test)
				}
			})
		}
		t.Logf("passed: %d; failed: %d; skipped: %d", tests.Passed, tests.Failed, tests.Skipped)
	}

	t.Run("decode", func(t *testing.T) { run(t, false) })
	t.Run("encode", func(t *testing.T) { run(t, true) })
}

func testMeta(t *testing.T, test tomltest.Test) {
	want, ok := metaTests[filepath.Base(test.Path)]
	if !ok {
		return
	}
	var s interface{}
	meta, err := toml.Decode(test.Input, &s)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	for _, k := range meta.Keys() {
		ks := k.String()
		b.WriteString(ks)
		b.WriteString(": ")
		b.WriteString(meta.Type(ks))
		b.WriteByte('\n')
	}
	have := b.String()
	have = have[:len(have)-1] // Trailing \n

	want = strings.ReplaceAll(strings.TrimSpace(want), "\t", "")
	if have != want {
		t.Errorf("MetaData wrong\nhave:\n%s\nwant:\n%s", have, want)
	}
}

func testError(t *testing.T, test tomltest.Test) {
	path := strings.TrimPrefix(test.Path, "invalid/")

	errs, ok := errorTests[path]
	if !ok {
		for k := range errorTests {
			ok, _ = filepath.Match(k, path)
			if ok {
				errs = errorTests[k]
				break
			}
		}
	}
	if !ok {
		return
	}

	for _, e := range errs {
		if !strings.Contains(test.Output, e) {
			t.Errorf("\nwrong error message\nhave: %s\nwant: %s", test.Output, e)
		}
	}
}

type parser struct{}

func (p parser) Encode(input string) (output string, outputIsError bool, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch rr := r.(type) {
			case error:
				retErr = rr
			default:
				retErr = fmt.Errorf("%s", rr)
			}
		}
	}()

	var tmp interface{}
	err := json.Unmarshal([]byte(input), &tmp)
	if err != nil {
		return "", false, err
	}

	buf := new(bytes.Buffer)
	err = toml.NewEncoder(buf).Encode(p.removeJSONTags(tmp))
	if err != nil {
		return err.Error(), true, retErr
	}

	return buf.String(), false, retErr
}

func (p parser) Decode(input string) (output string, outputIsError bool, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch rr := r.(type) {
			case error:
				retErr = rr
			default:
				retErr = fmt.Errorf("%s", rr)
			}
		}
	}()

	var d interface{}
	if _, err := toml.Decode(input, &d); err != nil {
		return err.Error(), true, retErr
	}

	j, err := json.MarshalIndent(p.addJSONTags(d), "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(j), false, retErr
}

func (p parser) addJSONTags(tomlData interface{}) interface{} {
	tag := func(typeName string, data interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":  typeName,
			"value": data,
		}
	}

	switch orig := tomlData.(type) {
	default:
		panic(fmt.Sprintf("Unknown type: %T", tomlData))

	case map[string]interface{}:
		typed := make(map[string]interface{}, len(orig))
		for k, v := range orig {
			typed[k] = p.addJSONTags(v)
		}
		return typed
	case []map[string]interface{}:
		typed := make([]map[string]interface{}, len(orig))
		for i, v := range orig {
			typed[i] = p.addJSONTags(v).(map[string]interface{})
		}
		return typed
	case []interface{}:
		typed := make([]interface{}, len(orig))
		for i, v := range orig {
			typed[i] = p.addJSONTags(v)
		}
		return typed
	case time.Time:
		return tag("datetime", orig.Format("2006-01-02T15:04:05.999999999Z07:00"))
	case bool:
		return tag("bool", fmt.Sprintf("%v", orig))
	case int64:
		return tag("integer", fmt.Sprintf("%d", orig))
	case float64:
		if math.IsNaN(orig) {
			return tag("float", "nan")
		}
		return tag("float", fmt.Sprintf("%v", orig))
	case string:
		return tag("string", orig)
	}
}

func (p parser) removeJSONTags(typedJson interface{}) interface{} {
	in := func(key string, m map[string]interface{}) bool {
		_, ok := m[key]
		return ok
	}

	untag := func(typed map[string]interface{}) interface{} {
		t := typed["type"].(string)
		v := typed["value"]
		switch t {
		case "string":
			return v.(string)
		case "integer":
			v := v.(string)
			n, err := strconv.Atoi(v)
			if err != nil {
				log.Fatalf("Could not parse '%s' as integer: %s", v, err)
			}
			return n
		case "float":
			v := v.(string)
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				log.Fatalf("Could not parse '%s' as float64: %s", v, err)
			}
			return f
		case "datetime":
			v := v.(string)
			t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", v)
			if err != nil {
				log.Fatalf("Could not parse '%s' as a datetime: %s", v, err)
			}
			return t
		case "bool":
			v := v.(string)
			switch v {
			case "true":
				return true
			case "false":
				return false
			}
			log.Fatalf("Could not parse '%s' as a boolean.", v)
		}
		log.Fatalf("Unrecognized tag type '%s'.", t)
		panic("unreachable")
	}

	switch v := typedJson.(type) {
	case map[string]interface{}:
		if len(v) == 2 && in("type", v) && in("value", v) {
			return untag(v)
		}
		m := make(map[string]interface{}, len(v))
		for k, v2 := range v {
			m[k] = p.removeJSONTags(v2)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(v))
		for i := range v {
			a[i] = p.removeJSONTags(v[i])
		}
		return a
	}
	log.Fatalf("Unrecognized JSON format '%T'.", typedJson)
	panic("unreachable")
}
