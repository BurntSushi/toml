package toml

import (
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"
)

func init() {
	log.SetFlags(0)
}

var testSimple = `
Andrew = "gallant"
Kait = "brady"
Now = 1987-07-05T05:45:00Z 
YesOrNo = true
Pi = 3.14
colors = [
	["red", "green", "blue"],
	["cyan", "magenta", "yellow", "black"],
]

[Cats]
Plato = "smelly"
Cauchy = "stupido"

`

var testMaps = `
#poop = "awesome"
#pee = "sweetness"

[Schools]
	[Schools.UMass]
	spent = 1

	[Schools.Worcester]
	spent = 3

	[Schools.Tufts]
	spent = 3
`

type kitties struct {
	Plato  string
	Cauchy string
}

type simple struct {
	Colors  [][]string `toml:"colors"`
	Pi      float64
	YesOrNo bool
	Now     time.Time
	Andrew  string
	Kait    string
	Cats    kitties
}

// Case insensitive matching tests.
// A bit more comprehensive than needed given the current implementation,
// but implementations change.
// Probably still missing demonstrations of some ugly corner cases regarding
// case insensitive matching and multiple fields.
var caseToml = `
tOpString = "string"
tOpInt = 1
tOpFloat = 1.1
tOpBool = true
tOpdate = 2006-01-02T15:04:05Z
tOparray = [ "array" ]
Match = "i should be in Match only"
MatcH = "i should be in MatcH only"
Field = "neat"
FielD = "messy"
once = "just once"
[nEst.eD]
nEstedString = "another string"
`

type Insensitive struct {
	TopString string
	TopInt    int
	TopFloat  float64
	TopBool   bool
	TopDate   time.Time
	TopArray  []string
	Match     string
	MatcH     string
	Field     string
	Once      string
	OncE      string
	Nest      InsensitiveNest
}
type InsensitiveNest struct {
	Ed InsensitiveEd
}
type InsensitiveEd struct {
	NestedString string
}

func TestCase(t *testing.T) {
	tme, err := time.Parse(time.RFC3339, time.RFC3339[:len(time.RFC3339)-5])
	if err != nil {
		panic(err)
	}
	expected := Insensitive{
		TopString: "string",
		TopInt:    1,
		TopFloat:  1.1,
		TopBool:   true,
		TopDate:   tme,
		TopArray:  []string{"array"},
		MatcH:     "i should be in MatcH only",
		Match:     "i should be in Match only",
		Field:     "neat", // encoding/json would store "messy" here
		Once:      "just once",
		OncE:      "just once", // wait, what?
		Nest:      InsensitiveNest{Ed: InsensitiveEd{NestedString: "another string"}},
	}
	var got Insensitive
	err = Decode(caseToml, &got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("\n%#v\n!=\n%#v\n", expected, got)
	}
}

func TestDecode(t *testing.T) {
	// var val = simple{
	// Andrew: "",
	// Kait: new(string),
	// }
	var val simple

	if err := Decode(testSimple, &val); err != nil {
		t.Fatal(err)
	}

	var schools map[string]map[string]map[string]int
	// var schools map[string]string
	if err := Decode(testMaps, &schools); err != nil {
		t.Fatal(err)
	}
}

func ExampleDecode() {
	var tomlBlob = `
# Some comments.
[alpha]
ip = "10.0.0.1"

	[alpha.config]
	Ports = [ 8001, 8002 ]
	Location = "Toronto"
	Created = 1987-07-05T05:45:00Z

[beta]
ip = "10.0.0.2"

	[beta.config]
	Ports = [ 9001, 9002 ]
	Location = "New Jersey"
	Created = 1887-01-05T05:55:00Z
`

	type serverConfig struct {
		Ports    []int
		Location string
		Created  time.Time
	}

	type server struct {
		IP     string       `toml:"ip"`
		Config serverConfig `toml:"config"`
	}

	type servers map[string]server

	var config servers
	if err := Decode(tomlBlob, &config); err != nil {
		log.Fatal(err)
	}

	for _, name := range []string{"alpha", "beta"} {
		s := config[name]
		fmt.Printf("Server: %s (ip: %s) in %s created on %s\n",
			name, s.IP, s.Config.Location,
			s.Config.Created.Format("2006-01-02"))
		fmt.Printf("Ports: %v\n", s.Config.Ports)
	}

	// Output:
	// Server: alpha (ip: 10.0.0.1) in Toronto created on 1987-07-05
	// Ports: [8001 8002]
	// Server: beta (ip: 10.0.0.2) in New Jersey created on 1887-01-05
	// Ports: [9001 9002]
}
