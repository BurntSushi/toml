package toml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEncodeRoundTrip(t *testing.T) {
	type Config struct {
		Age        int
		Cats       []string
		Pi         float64
		Perfection []int
		DOB        time.Time
		Ipaddress  net.IP
	}

	var inputs = Config{
		Age:        13,
		Cats:       []string{"one", "two", "three"},
		Pi:         3.145,
		Perfection: []int{11, 2, 3, 4},
		DOB:        time.Now(),
		Ipaddress:  net.ParseIP("192.168.59.254"),
	}

	var (
		firstBuffer  bytes.Buffer
		secondBuffer bytes.Buffer
		outputs      Config
	)
	err := NewEncoder(&firstBuffer).Encode(inputs)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Decode(firstBuffer.String(), &outputs)
	if err != nil {
		t.Logf("Could not decode:\n%s\n", firstBuffer.String())
		t.Fatal(err)
	}
	err = NewEncoder(&secondBuffer).Encode(outputs)
	if err != nil {
		t.Fatal(err)
	}
	if firstBuffer.String() != secondBuffer.String() {
		t.Errorf("%s\n\nIS NOT IDENTICAL TO\n\n%s", firstBuffer.String(), secondBuffer.String())
	}
}

func TestEncodeNestedTableArrays(t *testing.T) {
	type song struct {
		Name string `toml:"name"`
	}
	type album struct {
		Name  string `toml:"name"`
		Songs []song `toml:"songs"`
	}
	type springsteen struct {
		Albums []album `toml:"albums"`
	}
	value := springsteen{
		[]album{
			{"Born to Run",
				[]song{{"Jungleland"}, {"Meeting Across the River"}}},
			{"Born in the USA",
				[]song{{"Glory Days"}, {"Dancing in the Dark"}}},
		},
	}
	expected := `[[albums]]
  name = "Born to Run"

  [[albums.songs]]
    name = "Jungleland"

  [[albums.songs]]
    name = "Meeting Across the River"

[[albums]]
  name = "Born in the USA"

  [[albums.songs]]
    name = "Glory Days"

  [[albums.songs]]
    name = "Dancing in the Dark"
`
	encodeExpected(t, "nested table arrays", value, expected, nil)
}

func TestEncodeArrayHashWithNormalHashOrder(t *testing.T) {
	type Alpha struct {
		V int
	}
	type Beta struct {
		V int
	}
	type Conf struct {
		V int
		A Alpha
		B []Beta
	}

	val := Conf{
		V: 1,
		A: Alpha{2},
		B: []Beta{{3}},
	}
	expected := "V = 1\n\n[A]\n  V = 2\n\n[[B]]\n  V = 3\n"
	encodeExpected(t, "array hash with normal hash order", val, expected, nil)
}

func TestEncodeOmitEmptyStruct(t *testing.T) {
	type (
		T     struct{ Int int }
		Tpriv struct {
			Int     int
			private int
		}
		Ttime struct {
			Time time.Time
		}
	)

	tests := []struct {
		in   interface{}
		want string
	}{
		{struct {
			F T `toml:"f,omitempty"`
		}{}, ""},
		{struct {
			F T `toml:"f,omitempty"`
		}{T{1}}, "[f]\n  Int = 1"},

		{struct {
			F Tpriv `toml:"f,omitempty"`
		}{}, ""},
		{struct {
			F Tpriv `toml:"f,omitempty"`
		}{Tpriv{1, 0}}, "[f]\n  Int = 1"},

		// Private field being set also counts as "not empty".
		{struct {
			F Tpriv `toml:"f,omitempty"`
		}{Tpriv{0, 1}}, "[f]\n  Int = 0"},

		// time.Time is common use case, so test that explicitly.
		{struct {
			F Ttime `toml:"t,omitempty"`
		}{}, ""},
		{struct {
			F Ttime `toml:"t,omitempty"`
		}{Ttime{time.Time{}.Add(1)}}, "[t]\n  Time = 0001-01-01T00:00:00.000000001Z"},

		// TODO: also test with MarshalText, MarshalTOML returning non-zero
		// value.
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			buf := new(bytes.Buffer)

			err := NewEncoder(buf).Encode(tt.in)
			if err != nil {
				t.Fatal(err)
			}

			have := strings.TrimSpace(buf.String())
			if have != tt.want {
				t.Errorf("\nhave:\n%s\nwant:\n%s", have, tt.want)
			}
		})
	}
}

func TestEncodeWithOmitEmpty(t *testing.T) {
	type simple struct {
		Bool   bool              `toml:"bool,omitempty"`
		String string            `toml:"string,omitempty"`
		Array  [0]byte           `toml:"array,omitempty"`
		Slice  []int             `toml:"slice,omitempty"`
		Map    map[string]string `toml:"map,omitempty"`
		Time   time.Time         `toml:"time,omitempty"`
	}

	var v simple
	encodeExpected(t, "fields with omitempty are omitted when empty", v, "", nil)
	v = simple{
		Bool:   true,
		String: " ",
		Slice:  []int{2, 3, 4},
		Map:    map[string]string{"foo": "bar"},
		Time:   time.Date(1985, 6, 18, 15, 16, 17, 0, time.UTC),
	}
	expected := `bool = true
string = " "
slice = [2, 3, 4]
time = 1985-06-18T15:16:17Z

[map]
  foo = "bar"
`
	encodeExpected(t, "fields with omitempty are not omitted when non-empty",
		v, expected, nil)
}

func TestEncodeWithOmitZero(t *testing.T) {
	type simple struct {
		Number   int     `toml:"number,omitzero"`
		Real     float64 `toml:"real,omitzero"`
		Unsigned uint    `toml:"unsigned,omitzero"`
	}

	value := simple{0, 0.0, uint(0)}
	expected := ""

	encodeExpected(t, "simple with omitzero, all zero", value, expected, nil)

	value.Number = 10
	value.Real = 20
	value.Unsigned = 5
	expected = `number = 10
real = 20.0
unsigned = 5
`
	encodeExpected(t, "simple with omitzero, non-zero", value, expected, nil)
}

func TestEncodeOmitemptyWithEmptyName(t *testing.T) {
	type simple struct {
		S []int `toml:",omitempty"`
	}
	v := simple{[]int{1, 2, 3}}
	expected := "S = [1, 2, 3]\n"
	encodeExpected(t, "simple with omitempty, no name, non-empty field",
		v, expected, nil)
}

func TestEncodeAnonymousStruct(t *testing.T) {
	type Inner struct{ N int }
	type Outer0 struct{ Inner }
	type Outer1 struct {
		Inner `toml:"inner"`
	}

	v0 := Outer0{Inner{3}}
	expected := "N = 3\n"
	encodeExpected(t, "embedded anonymous untagged struct", v0, expected, nil)

	v1 := Outer1{Inner{3}}
	expected = "[inner]\n  N = 3\n"
	encodeExpected(t, "embedded anonymous tagged struct", v1, expected, nil)
}

func TestEncodeAnonymousStructPointerField(t *testing.T) {
	type Inner struct{ N int }
	type Outer0 struct{ *Inner }
	type Outer1 struct {
		*Inner `toml:"inner"`
	}

	v0 := Outer0{}
	expected := ""
	encodeExpected(t, "nil anonymous untagged struct pointer field", v0, expected, nil)

	v0 = Outer0{&Inner{3}}
	expected = "N = 3\n"
	encodeExpected(t, "non-nil anonymous untagged struct pointer field", v0, expected, nil)

	v1 := Outer1{}
	expected = ""
	encodeExpected(t, "nil anonymous tagged struct pointer field", v1, expected, nil)

	v1 = Outer1{&Inner{3}}
	expected = "[inner]\n  N = 3\n"
	encodeExpected(t, "non-nil anonymous tagged struct pointer field", v1, expected, nil)
}

func TestEncodeNestedAnonymousStructs(t *testing.T) {
	type A struct{ A string }
	type B struct{ B string }
	type C struct{ C string }
	type BC struct {
		B
		C
	}
	type Outer struct {
		A
		BC
	}

	v := &Outer{
		A: A{
			A: "a",
		},
		BC: BC{
			B: B{
				B: "b",
			},
			C: C{
				C: "c",
			},
		},
	}

	expected := "A = \"a\"\nB = \"b\"\nC = \"c\"\n"
	encodeExpected(t, "nested anonymous untagged structs", v, expected, nil)
}

func TestEncodeIgnoredFields(t *testing.T) {
	type simple struct {
		Number int `toml:"-"`
	}
	value := simple{}
	expected := ""
	encodeExpected(t, "ignored field", value, expected, nil)
}

func TestEncodeNaN(t *testing.T) {
	s1 := struct {
		Nan float64 `toml:"nan"`
		Inf float64 `toml:"inf"`
	}{math.NaN(), math.Inf(1)}
	s2 := struct {
		Nan float32 `toml:"nan"`
		Inf float32 `toml:"inf"`
	}{float32(math.NaN()), float32(math.Inf(-1))}
	encodeExpected(t, "", s1, "nan = nan\ninf = +inf\n", nil)
	encodeExpected(t, "", s2, "nan = nan\ninf = -inf\n", nil)
}

func TestEncodePrimitive(t *testing.T) {
	type MyStruct struct {
		Data  Primitive
		DataA int
		DataB string
	}

	decodeAndEncode := func(toml string) string {
		var s MyStruct
		_, err := Decode(toml, &s)
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		err = NewEncoder(&buf).Encode(s)
		if err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	original := `DataA = 1
DataB = "bbb"
Data = ["Foo", "Bar"]
`
	reEncoded := decodeAndEncode(decodeAndEncode(original))

	if reEncoded != original {
		t.Errorf(
			"re-encoded not the same as original\noriginal:   %q\nre-encoded: %q",
			original, reEncoded)
	}
}

func TestEncodeError(t *testing.T) {
	tests := []struct {
		in      interface{}
		wantErr string
	}{
		{make(chan int), "unsupported type for key '': chan"},
		{struct{ C complex128 }{0}, "unsupported type: complex128"},
		{[]complex128{0}, "unsupported type: complex128"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := NewEncoder(os.Stderr).Encode(tt.in)
			if err == nil {
				t.Fatal("err is nil")
			}
			if !errorContains(err, tt.wantErr) {
				t.Errorf("wrong error\nhave: %q\nwant: %q", err, tt.wantErr)
			}
		})
	}
}

type (
	sound struct{ S string }
	food  struct{ F []string }
	fun   func()
	cplx  complex128

	sound2 struct{ S string }
	food2  struct{ F []string }
	fun2   func()
	cplx2  complex128
)

// This is intentionally wrong (pointer receiver)
func (s *sound) MarshalText() ([]byte, error) { return []byte(s.S), nil }
func (f food) MarshalText() ([]byte, error)   { return []byte(strings.Join(f.F, ", ")), nil }
func (f fun) MarshalText() ([]byte, error)    { return []byte("why would you do this?"), nil }
func (c cplx) MarshalText() ([]byte, error) {
	cplx := complex128(c)
	return []byte(fmt.Sprintf("(%f+%fi)", real(cplx), imag(cplx))), nil
}

func (s *sound2) MarshalTOML() ([]byte, error) { return []byte("\"" + s.S + "\""), nil }
func (f food2) MarshalTOML() ([]byte, error) {
	return []byte("[\"" + strings.Join(f.F, "\", \"") + "\"]"), nil
}
func (f fun2) MarshalTOML() ([]byte, error) { return []byte("\"why would you do this?\""), nil }
func (c cplx2) MarshalTOML() ([]byte, error) {
	cplx := complex128(c)
	return []byte(fmt.Sprintf("\"(%f+%fi)\"", real(cplx), imag(cplx))), nil
}

func TestEncodeTextMarshaler(t *testing.T) {
	x := struct {
		Name    string
		Labels  map[string]string
		Sound   sound
		Sound2  *sound
		Food    food
		Food2   *food
		Complex cplx
		Fun     fun
	}{
		Name:   "Goblok",
		Sound:  sound{"miauw"},
		Sound2: &sound{"miauw"},
		Labels: map[string]string{
			"type":  "cat",
			"color": "black",
		},
		Food:    food{[]string{"chicken", "fish"}},
		Food2:   &food{[]string{"chicken", "fish"}},
		Complex: complex(42, 666),
		Fun:     func() { panic("x") },
	}

	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(x); err != nil {
		t.Fatal(err)
	}

	want := `Name = "Goblok"
Sound2 = "miauw"
Food = "chicken, fish"
Food2 = "chicken, fish"
Complex = "(42.000000+666.000000i)"
Fun = "why would you do this?"

[Labels]
  color = "black"
  type = "cat"

[Sound]
  S = "miauw"
`

	if buf.String() != want {
		t.Error("\n" + buf.String())
	}
}

func TestEncodeTOMLMarshaler(t *testing.T) {
	x := struct {
		Name    string
		Labels  map[string]string
		Sound   sound2
		Sound2  *sound2
		Food    food2
		Food2   *food2
		Complex cplx2
		Fun     fun2
	}{
		Name:   "Goblok",
		Sound:  sound2{"miauw"},
		Sound2: &sound2{"miauw"},
		Labels: map[string]string{
			"type":  "cat",
			"color": "black",
		},
		Food:    food2{[]string{"chicken", "fish"}},
		Food2:   &food2{[]string{"chicken", "fish"}},
		Complex: complex(42, 666),
		Fun:     func() { panic("x") },
	}

	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(x); err != nil {
		t.Fatal(err)
	}

	want := `Name = "Goblok"
Sound2 = "miauw"
Food = ["chicken", "fish"]
Food2 = ["chicken", "fish"]
Complex = "(42.000000+666.000000i)"
Fun = "why would you do this?"

[Labels]
  color = "black"
  type = "cat"

[Sound]
  S = "miauw"
`

	if buf.String() != want {
		t.Error("\n" + buf.String())
	}
}

type (
	retNil1 string
	retNil2 string
)

func (r retNil1) MarshalText() ([]byte, error) { return nil, nil }
func (r retNil2) MarshalTOML() ([]byte, error) { return nil, nil }

func TestEncodeEmpty(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		var (
			s   struct{ Text retNil1 }
			buf bytes.Buffer
		)
		err := NewEncoder(&buf).Encode(s)
		if err == nil {
			t.Fatalf("no error, but expected an error; output:\n%s", buf.String())
		}
		if buf.String() != "" {
			t.Error("\n" + buf.String())
		}
	})

	t.Run("toml", func(t *testing.T) {
		var (
			s   struct{ Text retNil2 }
			buf bytes.Buffer
		)
		err := NewEncoder(&buf).Encode(s)
		if err == nil {
			t.Fatalf("no error, but expected an error; output:\n%s", buf.String())
		}
		if buf.String() != "" {
			t.Error("\n" + buf.String())
		}
	})
}

// Would previously fail on 32bit architectures; can test with:
//   GOARCH=386         go test -c &&  ./toml.test
//   GOARCH=arm GOARM=7 go test -c && qemu-arm ./toml.test
func TestEncode32bit(t *testing.T) {
	type Inner struct {
		A, B, C string
	}
	type Outer struct{ Inner }

	encodeExpected(t, "embedded anonymous untagged struct",
		Outer{Inner{"a", "b", "c"}},
		"A = \"a\"\nB = \"b\"\nC = \"c\"\n",
		nil)
}

// Skip invalid types if it has toml:"-"
//
// https://github.com/BurntSushi/toml/issues/345
func TestEncodeSkipInvalidType(t *testing.T) {
	buf := new(bytes.Buffer)
	err := NewEncoder(buf).Encode(struct {
		Str  string                 `toml:"str"`
		Arr  []func()               `toml:"-"`
		Map  map[string]interface{} `toml:"-"`
		Func func()                 `toml:"-"`
	}{
		Str:  "a",
		Arr:  []func(){func() {}},
		Map:  map[string]interface{}{"f": func() {}},
		Func: func() {},
	})
	if err != nil {
		t.Fatal(err)
	}

	have := buf.String()
	want := "str = \"a\"\n"
	if have != want {
		t.Errorf("\nwant: %q\nhave: %q\n", want, have)
	}
}

func TestEncodeDuration(t *testing.T) {
	tests := []time.Duration{
		0,
		time.Second,
		time.Minute,
		time.Hour,
		248*time.Hour + 45*time.Minute + 24*time.Second,
		12345678 * time.Nanosecond,
		12345678 * time.Second,
		4*time.Second + 2*time.Nanosecond,
	}

	for _, tt := range tests {
		encodeExpected(t, tt.String(),
			struct{ Dur time.Duration }{Dur: tt},
			fmt.Sprintf("Dur = %q", tt), nil)
	}
}

type jsonT struct {
	Num  json.Number
	NumP *json.Number
	Arr  []json.Number
	ArrP []*json.Number
	Tbl  map[string]json.Number
	TblP map[string]*json.Number
}

var (
	n2, n4, n6 = json.Number("2"), json.Number("4"), json.Number("6")
	f2, f4, f6 = json.Number("2.2"), json.Number("4.4"), json.Number("6.6")
)

func TestEncodeJSONNumber(t *testing.T) {
	tests := []struct {
		in   jsonT
		want string
	}{
		{jsonT{}, "Num = 0"},
		{jsonT{
			Num:  "1",
			NumP: &n2,
			Arr:  []json.Number{"3"},
			ArrP: []*json.Number{&n4},
			Tbl:  map[string]json.Number{"k1": "5"},
			TblP: map[string]*json.Number{"k2": &n6}}, `
				Num = 1
				NumP = 2
				Arr = [3]
				ArrP = [4]

				[Tbl]
				  k1 = 5

				[TblP]
				  k2 = 6
		`},
		{jsonT{
			Num:  "1.1",
			NumP: &f2,
			Arr:  []json.Number{"3.3"},
			ArrP: []*json.Number{&f4},
			Tbl:  map[string]json.Number{"k1": "5.5"},
			TblP: map[string]*json.Number{"k2": &f6}}, `
				Num = 1.1
				NumP = 2.2
				Arr = [3.3]
				ArrP = [4.4]

				[Tbl]
				  k1 = 5.5

				[TblP]
				  k2 = 6.6
		`},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var buf bytes.Buffer
			err := NewEncoder(&buf).Encode(tt.in)
			if err != nil {
				t.Fatal(err)
			}

			have := strings.TrimSpace(buf.String())
			want := strings.ReplaceAll(strings.TrimSpace(tt.want), "\t", "")
			if have != want {
				t.Errorf("\nwant:\n%s\nhave:\n%s\n", want, have)
			}
		})
	}
}

func encodeExpected(t *testing.T, label string, val interface{}, want string, wantErr error) {
	t.Helper()

	t.Run(label, func(t *testing.T) {
		var buf bytes.Buffer
		err := NewEncoder(&buf).Encode(val)
		if err != wantErr {
			if wantErr != nil {
				if wantErr == errAnything && err != nil {
					return
				}
				t.Errorf("want Encode error %v, got %v", wantErr, err)
			} else {
				t.Errorf("Encode failed: %s", err)
			}
		}
		if err != nil {
			return
		}

		have := strings.TrimSpace(buf.String())
		want = strings.TrimSpace(want)
		if want != have {
			t.Errorf("\nhave:\n%s\nwant:\n%s\n", have, want)
		}
	})
}
