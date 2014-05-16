package toml

import (
	"bytes"
	"net"
	"testing"
	"time"
)

type Config struct {
	Age        int
	Cats       []string
	Pi         float64
	Perfection []int
	DOB        time.Time // requires `import time`
	Ipaddress  net.IP    // requires `import net`
}

func TestRoundTrip(t *testing.T) {
	var inputs = Config{
		13,
		[]string{"one", "two", "three"},
		3.145,
		[]int{11, 2, 3, 4},
		time.Now(),
		net.ParseIP("192.168.59.254"),
	}

	var firstBuffer bytes.Buffer
	e := NewEncoder(&firstBuffer)
	err := e.Encode(inputs)
	if err != nil {
		t.Fatal(err)
	}
	var outputs Config
	if _, err := Decode(firstBuffer.String(), &outputs); err != nil {
		t.Fatal(err)
	}

	// could test each value individually, but I'm lazy
	var secondBuffer bytes.Buffer
	e2 := NewEncoder(&secondBuffer)
	err = e2.Encode(outputs)
	if err != nil {
		t.Fatal(err)
	}
	if firstBuffer.String() != secondBuffer.String() {
		t.Error(
			firstBuffer.String(),
			"\n\n is not identical to\n\n",
			secondBuffer.String())
	}
}
