package toml

import (
	"fmt"
	"log"
	"strings"
	"testing"
)

func init() {
	log.SetFlags(0)
}

var testSmall2 = `
# This is a TOML document. Boom.

wat = "chipper"

[owner.andrew.gallant] 
hmm = "hi"

[owner] # Whoa there.
andreW = "gallant # poopy" # weeeee
predicate = false
num = -5192
f = -0.5192
zulu = 1979-05-27T07:32:00Z
whoop = "poop"
tests = [ [1, 2, 3], ["abc", "xyz"] ]
arrs = [ # hmm
		 # more comments are awesome.
	1987-07-05T05:45:00Z,
	# say wat?
	1987-07-05T05:45:00Z,
	1987-07-05T05:45:00Z,
	# sweetness
] # more comments
# hehe
`

func TestParse(t *testing.T) {
	_, err := parse(testSmall2)
	if err != nil {
		t.Fatal(err)
	}
}

func printMap(m map[string]interface{}, depth int) {
	for k, v := range m {
		fmt.Printf("%s%s\n", strings.Repeat("  ", depth), k)
		switch subm := v.(type) {
		case map[string]interface{}:
			printMap(subm, depth+1)
		default:
			fmt.Printf("%s%v\n", strings.Repeat("  ", depth+1), v)
		}
	}
}
