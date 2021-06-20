// Command toml-test-decoder satisfies the toml-test interface for testing TOML
// decoders. Namely, it accepts TOML on stdin and outputs JSON on stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"time"

	"github.com/BurntSushi/toml"
)

func init() {
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
}

func usage() {
	log.Printf("Usage: %s < toml-file\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	if flag.NArg() != 0 {
		flag.Usage()
	}

	var decoded interface{}
	if _, err := toml.DecodeReader(os.Stdin, &decoded); err != nil {
		log.Fatalf("Error decoding TOML: %s", err)
	}

	j := json.NewEncoder(os.Stdout)
	j.SetIndent("", "  ")
	if err := j.Encode(addJSONTags(decoded)); err != nil {
		log.Fatalf("Error encoding JSON: %s", err)
	}
}

func addJSONTags(tomlData interface{}) interface{} {
	switch orig := tomlData.(type) {
	default:
		panic(fmt.Sprintf("Unknown type: %T", tomlData))

	case map[string]interface{}:
		typed := make(map[string]interface{}, len(orig))
		for k, v := range orig {
			typed[k] = addJSONTags(v)
		}
		return typed
	case []map[string]interface{}:
		typed := make([]map[string]interface{}, len(orig))
		for i, v := range orig {
			typed[i] = addJSONTags(v).(map[string]interface{})
		}
		return typed
	case []interface{}:
		typed := make([]interface{}, len(orig))
		for i, v := range orig {
			typed[i] = addJSONTags(v)
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

func tag(typeName string, data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":  typeName,
		"value": data,
	}
}
