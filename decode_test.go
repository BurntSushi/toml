package toml

import (
	"fmt"
	"log"
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
