// Command toml-test-encoder satisfies the toml-test interface for testing TOML
// encoders. Namely, it accepts JSON on stdin and outputs TOML on stdout.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

func init() {
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
}

func usage() {
	log.Printf("Usage: %s < json-file\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	if flag.NArg() != 0 {
		flag.Usage()
	}

	var tmp interface{}
	if err := json.NewDecoder(os.Stdin).Decode(&tmp); err != nil {
		log.Fatalf("Error decoding JSON: %s", err)
	}

	if err := toml.NewEncoder(os.Stdout).Encode(translate(tmp)); err != nil {
		log.Fatalf("Error encoding TOML: %s", err)
	}
}

func translate(typedJson interface{}) interface{} {
	switch v := typedJson.(type) {
	case map[string]interface{}:
		if len(v) == 2 && in("type", v) && in("value", v) {
			return untag(v)
		}
		m := make(map[string]interface{}, len(v))
		for k, v2 := range v {
			m[k] = translate(v2)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(v))
		for i := range v {
			a[i] = translate(v[i])
		}
		return a
	}
	log.Fatalf("Unrecognized JSON format '%T'.", typedJson)
	panic("unreachable")
}

func untag(typed map[string]interface{}) interface{} {
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

func in(key string, m map[string]interface{}) bool {
	_, ok := m[key]
	return ok
}
