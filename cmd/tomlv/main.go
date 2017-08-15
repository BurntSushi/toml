// Command tomlv validates TOML documents and prints each key's type.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	"github.com/BurntSushi/toml"
)

var (
	flagTypes = false
	getValue  = ""
)

func init() {
	log.SetFlags(0)

	flag.BoolVar(&flagTypes, "types", flagTypes,
		"When set, the types of every defined key will be shown.")

	flag.StringVar(&getValue, "get", getValue,
		"When set will attempt to retrieve the value of the key specified.")

	flag.Usage = usage
	flag.Parse()
}

func usage() {
	log.Printf("Usage: %s toml-file [ toml-file ... ]\n",
		path.Base(os.Args[0]))
	flag.PrintDefaults()

	os.Exit(1)
}

func main() {
	if flag.NArg() < 1 {
		flag.Usage()
	}
	for _, f := range flag.Args() {
		var tmp interface{}
		md, err := toml.DecodeFile(f, &tmp)
		if err != nil {
			log.Fatalf("Error in '%s': %s", f, err)
		}
		if flagTypes {
			printTypes(md)
		}

		if getValue != "" {
			getVal(getValue, md, tmp)
		}
	}
}

func printTypes(md toml.MetaData) {
	tabw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, key := range md.Keys() {
		fmt.Fprintf(tabw, "%s%s\t%s\n",
			strings.Repeat("    ", len(key)-1), key, md.Type(key...))
	}
	tabw.Flush()
}

func getVal(value string, md toml.MetaData, data interface{}) {
	var val interface{}
	var typedData map[string]interface{}

	typedData, ok := data.(map[string]interface{})
	if !ok {
		fmt.Println("We had an unexpected error. Possible the toml is malformed?")
		os.Exit(1)
	}

	split := strings.Split(value, ".")
	for _, s := range split {
		val = typedData[s]

		if maybeTyped, ok := val.(map[string]interface{}); ok {
			typedData = maybeTyped
		}
	}

	if val == nil {
		fmt.Println("no key with that name exists")
		os.Exit(0)
	}

	fmt.Println(val)
}
