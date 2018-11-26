package main

import (
	"fmt"
	"github.com/troian/toml"
	"os"
)

type config struct {
	Val1 string `toml:"val1" comment:"test comment 1"`
	Val2 string `toml:"val2" comment:"test comment 2\nnext line"`
	Val3 string `toml:"val3" commented:"true"`
	Val4 string `toml:"val4,omitempty" commented:"true"`
}

func main () {
	var cfg config

	if err := toml.NewEncoder(os.Stdout).Encode(&cfg); err != nil {
		fmt.Println(err)
		return
	}
}
