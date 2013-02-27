package main

import (
	"flag"
	"log"
	"os"
	"path"

	"github.com/BurntSushi/toml"
)

func init() {
	log.SetFlags(0)

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
		if err := toml.DecodeFile(f, &tmp); err != nil {
			log.Fatalf("Error in '%s': %s", f, err)
		}
	}
}
