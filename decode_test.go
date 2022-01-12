package toml

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml/internal"
)

func TestDecodeReader(t *testing.T) {
	var i struct{ A int }
	meta, err := DecodeReader(strings.NewReader("a = 42"), &i)
	if err != nil {
		t.Fatal(err)
	}
	have := fmt.Sprintf("%v %v %v", i, meta.Keys(), meta.Type("a"))
	want := "{42} [a] Integer"
	if have != want {
		t.Errorf("\nhave: %s\nwant: %s", have, want)
	}
}

func TestDecodeFile(t *testing.T) {
	tmp, err := ioutil.TempFile("", "toml-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString("a = 42"); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}

	var i struct{ A int }
	meta, err := DecodeFile(tmp.Name(), &i)
	if err != nil {
		t.Fatal(err)
	}

	have := fmt.Sprintf("%v %v %v", i, meta.Keys(), meta.Type("a"))
	want := "{42} [a] Integer"
	if have != want {
		t.Errorf("\nhave: %s\nwant: %s", have, want)
	}
}

func TestDecodeBOM(t *testing.T) {
	for _, tt := range [][]byte{
		[]byte("\xff\xfea = \"b\""),
		[]byte("\xfe\xffa = \"b\""),
	} {
		t.Run("", func(t *testing.T) {
			var s struct{ A string }
			_, err := Decode(string(tt), &s)
			if err != nil {
				t.Fatal(err)
			}
			if s.A != "b" {
				t.Errorf(`s.A is not "b" but %q`, s.A)
			}
		})
	}
}

func TestDecodeEmbedded(t *testing.T) {
	type Dog struct{ Name string }
	type Age int
	type cat struct{ Name string }

	for _, test := range []struct {
		label       string
		input       string
		decodeInto  interface{}
		wantDecoded interface{}
	}{
		{
			label:       "embedded struct",
			input:       `Name = "milton"`,
			decodeInto:  &struct{ Dog }{},
			wantDecoded: &struct{ Dog }{Dog{"milton"}},
		},
		{
			label:       "embedded non-nil pointer to struct",
			input:       `Name = "milton"`,
			decodeInto:  &struct{ *Dog }{},
			wantDecoded: &struct{ *Dog }{&Dog{"milton"}},
		},
		{
			label:       "embedded nil pointer to struct",
			input:       ``,
			decodeInto:  &struct{ *Dog }{},
			wantDecoded: &struct{ *Dog }{nil},
		},
		{
			label:       "unexported embedded struct",
			input:       `Name = "socks"`,
			decodeInto:  &struct{ cat }{},
			wantDecoded: &struct{ cat }{cat{"socks"}},
		},
		{
			label:       "embedded int",
			input:       `Age = -5`,
			decodeInto:  &struct{ Age }{},
			wantDecoded: &struct{ Age }{-5},
		},
	} {
		_, err := Decode(test.input, test.decodeInto)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(test.wantDecoded, test.decodeInto) {
			t.Errorf("%s: want decoded == %+v, got %+v",
				test.label, test.wantDecoded, test.decodeInto)
		}
	}
}

func TestDecodeIgnoreFields(t *testing.T) {
	const input = `
Number = 123
- = 234
`
	var s struct {
		Number int `toml:"-"`
	}
	if _, err := Decode(input, &s); err != nil {
		t.Fatal(err)
	}
	if s.Number != 0 {
		t.Errorf("got: %d; want 0", s.Number)
	}
}

func TestDecodeTableArrays(t *testing.T) {
	var tomlTableArrays = `
[[albums]]
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

	type Song struct {
		Name string
	}

	type Album struct {
		Name  string
		Songs []Song
	}

	type Music struct {
		Albums []Album
	}

	expected := Music{[]Album{
		{"Born to Run", []Song{{"Jungleland"}, {"Meeting Across the River"}}},
		{"Born in the USA", []Song{{"Glory Days"}, {"Dancing in the Dark"}}},
	}}
	var got Music
	if _, err := Decode(tomlTableArrays, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("\n%#v\n!=\n%#v\n", expected, got)
	}
}

func TestDecodePointers(t *testing.T) {
	type Object struct {
		Type        string
		Description string
	}

	type Dict struct {
		NamedObject map[string]*Object
		BaseObject  *Object
		Strptr      *string
		Strptrs     []*string
	}
	s1, s2, s3 := "blah", "abc", "def"
	expected := &Dict{
		Strptr:  &s1,
		Strptrs: []*string{&s2, &s3},
		NamedObject: map[string]*Object{
			"foo": {"FOO", "fooooo!!!"},
			"bar": {"BAR", "ba-ba-ba-ba-barrrr!!!"},
		},
		BaseObject: &Object{"BASE", "da base"},
	}

	ex1 := `
Strptr = "blah"
Strptrs = ["abc", "def"]

[NamedObject.foo]
Type = "FOO"
Description = "fooooo!!!"

[NamedObject.bar]
Type = "BAR"
Description = "ba-ba-ba-ba-barrrr!!!"

[BaseObject]
Type = "BASE"
Description = "da base"
`
	dict := new(Dict)
	_, err := Decode(ex1, dict)
	if err != nil {
		t.Errorf("Decode error: %v", err)
	}
	if !reflect.DeepEqual(expected, dict) {
		t.Fatalf("\n%#v\n!=\n%#v\n", expected, dict)
	}
}

func TestDecodeBadDatetime(t *testing.T) {
	var x struct{ T time.Time }
	for _, s := range []string{"123", "1230"} {
		input := "T = " + s
		if _, err := Decode(input, &x); err == nil {
			t.Errorf("Expected invalid DateTime error for %q", s)
		}
	}
}

type sphere struct {
	Center [3]float64
	Radius float64
}

func TestDecodeArrayWrongSize(t *testing.T) {
	var s1 sphere
	if _, err := Decode(`center = [0.1, 2.3]`, &s1); err == nil {
		t.Fatal("Expected array type mismatch error")
	}
}

func TestDecodeIntOverflow(t *testing.T) {
	type table struct {
		Value int8
	}
	var tab table
	if _, err := Decode(`value = 500`, &tab); err == nil {
		t.Fatal("Expected integer out-of-bounds error.")
	}
}

func TestDecodeFloatOverflow(t *testing.T) {
	tests := []struct {
		value    string
		overflow bool
	}{
		{fmt.Sprintf(`F32 = %f`, math.MaxFloat64), true},
		{fmt.Sprintf(`F32 = %f`, -math.MaxFloat64), true},
		{fmt.Sprintf(`F32 = %f`, math.MaxFloat32*1.1), true},
		{fmt.Sprintf(`F32 = %f`, -math.MaxFloat32*1.1), true},
		{fmt.Sprintf(`F32 = %d`, maxSafeFloat32Int+1), true},
		{fmt.Sprintf(`F32 = %d`, -maxSafeFloat32Int-1), true},
		{fmt.Sprintf(`F64 = %d`, maxSafeFloat64Int+1), true},
		{fmt.Sprintf(`F64 = %d`, -maxSafeFloat64Int-1), true},

		{fmt.Sprintf(`F32 = %f`, math.MaxFloat32), false},
		{fmt.Sprintf(`F32 = %f`, -math.MaxFloat32), false},
		{fmt.Sprintf(`F32 = %d`, maxSafeFloat32Int), false},
		{fmt.Sprintf(`F32 = %d`, -maxSafeFloat32Int), false},
		{fmt.Sprintf(`F64 = %f`, math.MaxFloat64), false},
		{fmt.Sprintf(`F64 = %f`, -math.MaxFloat64), false},
		{fmt.Sprintf(`F64 = %f`, math.MaxFloat32), false},
		{fmt.Sprintf(`F64 = %f`, -math.MaxFloat32), false},
		{fmt.Sprintf(`F64 = %d`, maxSafeFloat64Int), false},
		{fmt.Sprintf(`F64 = %d`, -maxSafeFloat64Int), false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var tab struct {
				F32 float32
				F64 float64
			}
			_, err := Decode(tt.value, &tab)

			if tt.overflow && err == nil {
				t.Fatal("expected error, but err is nil")
			}
			if (tt.overflow && !errorContains(err, "out of range")) || (!tt.overflow && err != nil) {
				t.Fatalf("unexpected error:\n%v", err)
			}
		})
	}
}

func TestDecodeSizedInts(t *testing.T) {
	type table struct {
		U8  uint8
		U16 uint16
		U32 uint32
		U64 uint64
		U   uint
		I8  int8
		I16 int16
		I32 int32
		I64 int64
		I   int
	}
	answer := table{1, 1, 1, 1, 1, -1, -1, -1, -1, -1}
	toml := `
	u8 = 1
	u16 = 1
	u32 = 1
	u64 = 1
	u = 1
	i8 = -1
	i16 = -1
	i32 = -1
	i64 = -1
	i = -1
	`
	var tab table
	if _, err := Decode(toml, &tab); err != nil {
		t.Fatal(err.Error())
	}
	if answer != tab {
		t.Fatalf("Expected %#v but got %#v", answer, tab)
	}
}

type NopUnmarshalTOML int

func (NopUnmarshalTOML) UnmarshalTOML(p interface{}) error { return nil }

func TestDecodeTypes(t *testing.T) {
	type mystr string

	for _, tt := range []struct {
		v    interface{}
		want string
	}{
		{new(map[string]bool), ""},
		{new(map[mystr]bool), ""},
		{new(NopUnmarshalTOML), ""},

		{3, `toml: cannot decode to non-pointer "int"`},
		{map[string]interface{}{}, `toml: cannot decode to non-pointer "map[string]interface {}"`},

		{(*int)(nil), `toml: cannot decode to nil value of "*int"`},
		{(*Unmarshaler)(nil), `toml: cannot decode to nil value of "*toml.Unmarshaler"`},
		{nil, `toml: cannot decode to non-pointer <nil>`},

		{new(map[int]string), "cannot decode to a map with non-string key type"},
		{new(map[interface{}]string), "cannot decode to a map with non-string key type"},

		{new(struct{ F int }), `toml: incompatible types: TOML key "F" has type bool; destination has type integer`},
		{new(map[string]int), `toml: incompatible types: TOML key "F" has type bool; destination has type integer`},
		{new(int), `toml: cannot decode to type int`},
		{new([]int), "toml: cannot decode to type []int"},
	} {
		t.Run(fmt.Sprintf("%T", tt.v), func(t *testing.T) {
			_, err := Decode(`F = true`, tt.v)
			if !errorContains(err, tt.want) {
				t.Errorf("wrong error\nhave: %q\nwant: %q", err, tt.want)
			}
		})
	}
}

func TestUnmarshaler(t *testing.T) {
	var tomlBlob = `
[dishes.hamboogie]
name = "Hamboogie with fries"
price = 10.99

[[dishes.hamboogie.ingredients]]
name = "Bread Bun"

[[dishes.hamboogie.ingredients]]
name = "Lettuce"

[[dishes.hamboogie.ingredients]]
name = "Real Beef Patty"

[[dishes.hamboogie.ingredients]]
name = "Tomato"

[dishes.eggsalad]
name = "Egg Salad with rice"
price = 3.99

[[dishes.eggsalad.ingredients]]
name = "Egg"

[[dishes.eggsalad.ingredients]]
name = "Mayo"

[[dishes.eggsalad.ingredients]]
name = "Rice"
`
	m := &menu{}
	if _, err := Decode(tomlBlob, m); err != nil {
		t.Fatal(err)
	}

	if len(m.Dishes) != 2 {
		t.Log("two dishes should be loaded with UnmarshalTOML()")
		t.Errorf("expected %d but got %d", 2, len(m.Dishes))
	}

	eggSalad := m.Dishes["eggsalad"]
	if _, ok := interface{}(eggSalad).(dish); !ok {
		t.Errorf("expected a dish")
	}

	if eggSalad.Name != "Egg Salad with rice" {
		t.Errorf("expected the dish to be named 'Egg Salad with rice'")
	}

	if len(eggSalad.Ingredients) != 3 {
		t.Log("dish should be loaded with UnmarshalTOML()")
		t.Errorf("expected %d but got %d", 3, len(eggSalad.Ingredients))
	}

	found := false
	for _, i := range eggSalad.Ingredients {
		if i.Name == "Rice" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Rice was not loaded in UnmarshalTOML()")
	}

	// test on a value - must be passed as *
	o := menu{}
	if _, err := Decode(tomlBlob, &o); err != nil {
		t.Fatal(err)
	}

}

func TestDecodeInlineTable(t *testing.T) {
	input := `
[CookieJar]
Types = {Chocolate = "yummy", Oatmeal = "best ever"}

[Seasons]
Locations = {NY = {Temp = "not cold", Rating = 4}, MI = {Temp = "freezing", Rating = 9}}
`
	type cookieJar struct {
		Types map[string]string
	}
	type properties struct {
		Temp   string
		Rating int
	}
	type seasons struct {
		Locations map[string]properties
	}
	type wrapper struct {
		CookieJar cookieJar
		Seasons   seasons
	}
	var got wrapper

	meta, err := Decode(input, &got)
	if err != nil {
		t.Fatal(err)
	}
	want := wrapper{
		CookieJar: cookieJar{
			Types: map[string]string{
				"Chocolate": "yummy",
				"Oatmeal":   "best ever",
			},
		},
		Seasons: seasons{
			Locations: map[string]properties{
				"NY": {
					Temp:   "not cold",
					Rating: 4,
				},
				"MI": {
					Temp:   "freezing",
					Rating: 9,
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after decode, got:\n\n%#v\n\nwant:\n\n%#v", got, want)
	}
	if len(meta.keys) != 12 {
		t.Errorf("after decode, got %d meta keys; want 12", len(meta.keys))
	}
	if len(meta.types) != 12 {
		t.Errorf("after decode, got %d meta types; want 12", len(meta.types))
	}
}

func TestDecodeInlineTableArray(t *testing.T) {
	type point struct {
		X, Y, Z int
	}
	var got struct {
		Points []point
	}
	// Example inline table array from the spec.
	const in = `
points = [ { x = 1, y = 2, z = 3 },
           { x = 7, y = 8, z = 9 },
           { x = 2, y = 4, z = 8 } ]

`
	if _, err := Decode(in, &got); err != nil {
		t.Fatal(err)
	}
	want := []point{
		{X: 1, Y: 2, Z: 3},
		{X: 7, Y: 8, Z: 9},
		{X: 2, Y: 4, Z: 8},
	}
	if !reflect.DeepEqual(got.Points, want) {
		t.Errorf("got %#v; want %#v", got.Points, want)
	}
}

type menu struct {
	Dishes map[string]dish
}

func (m *menu) UnmarshalTOML(p interface{}) error {
	m.Dishes = make(map[string]dish)
	data, _ := p.(map[string]interface{})
	dishes := data["dishes"].(map[string]interface{})
	for n, v := range dishes {
		if d, ok := v.(map[string]interface{}); ok {
			nd := dish{}
			nd.UnmarshalTOML(d)
			m.Dishes[n] = nd
		} else {
			return fmt.Errorf("not a dish")
		}
	}
	return nil
}

type dish struct {
	Name        string
	Price       float32
	Ingredients []ingredient
}

func (d *dish) UnmarshalTOML(p interface{}) error {
	data, _ := p.(map[string]interface{})
	d.Name, _ = data["name"].(string)
	d.Price, _ = data["price"].(float32)
	ingredients, _ := data["ingredients"].([]map[string]interface{})
	for _, e := range ingredients {
		n, _ := interface{}(e).(map[string]interface{})
		name, _ := n["name"].(string)
		i := ingredient{name}
		d.Ingredients = append(d.Ingredients, i)
	}
	return nil
}

type ingredient struct {
	Name string
}

func TestDecodeSlices(t *testing.T) {
	type (
		T struct {
			Arr []string
			Tbl map[string]interface{}
		}
		M map[string]interface{}
	)
	tests := []struct {
		input    string
		in, want T
	}{
		{"",
			T{}, T{}},

		// Leave existing values alone.
		{"",
			T{[]string{}, M{"arr": []string{}}},
			T{[]string{}, M{"arr": []string{}}}},
		{"",
			T{[]string{"a"}, M{"arr": []string{"b"}}},
			T{[]string{"a"}, M{"arr": []string{"b"}}}},

		// Empty array always allocates (see #339)
		{`arr = []
		tbl = {arr = []}`,
			T{},
			T{[]string{}, M{"arr": []interface{}{}}}},
		{`arr = []
		tbl = {}`,
			T{[]string{}, M{}},
			T{[]string{}, M{}}},

		{`arr = []`,
			T{[]string{"a"}, M{}},
			T{[]string{}, M{}}},

		{`arr = ["x"]
		 tbl = {arr=["y"]}`,
			T{},
			T{[]string{"x"}, M{"arr": []interface{}{"y"}}}},
		{`arr = ["x"]
		 tbl = {arr=["y"]}`,
			T{[]string{}, M{}},
			T{[]string{"x"}, M{"arr": []interface{}{"y"}}}},
		{`arr = ["x"]
		tbl = {arr=["y"]}`,
			T{[]string{"a", "b"}, M{"arr": []interface{}{"c", "d"}}},
			T{[]string{"x"}, M{"arr": []interface{}{"y"}}}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			_, err := Decode(tt.input, &tt.in)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(tt.in, tt.want) {
				t.Errorf("\nhave: %#v\nwant: %#v", tt.in, tt.want)
			}
		})
	}
}

func TestDecodePrimitive(t *testing.T) {
	type S struct {
		P Primitive
	}
	type T struct {
		S []int
	}
	slicep := func(s []int) *[]int { return &s }
	arrayp := func(a [2]int) *[2]int { return &a }
	mapp := func(m map[string]int) *map[string]int { return &m }
	for i, tt := range []struct {
		v     interface{}
		input string
		want  interface{}
	}{
		// slices
		{slicep(nil), "", slicep(nil)},
		{slicep([]int{}), "", slicep([]int{})},
		{slicep([]int{1, 2, 3}), "", slicep([]int{1, 2, 3})},
		{slicep(nil), "P = [1,2]", slicep([]int{1, 2})},
		{slicep([]int{}), "P = [1,2]", slicep([]int{1, 2})},
		{slicep([]int{1, 2, 3}), "P = [1,2]", slicep([]int{1, 2})},

		// arrays
		{arrayp([2]int{2, 3}), "", arrayp([2]int{2, 3})},
		{arrayp([2]int{2, 3}), "P = [3,4]", arrayp([2]int{3, 4})},

		// maps
		{mapp(nil), "", mapp(nil)},
		{mapp(map[string]int{}), "", mapp(map[string]int{})},
		{mapp(map[string]int{"a": 1}), "", mapp(map[string]int{"a": 1})},
		{mapp(nil), "[P]\na = 2", mapp(map[string]int{"a": 2})},
		{mapp(map[string]int{}), "[P]\na = 2", mapp(map[string]int{"a": 2})},
		{mapp(map[string]int{"a": 1, "b": 3}), "[P]\na = 2", mapp(map[string]int{"a": 2, "b": 3})},

		// structs
		{&T{nil}, "[P]", &T{nil}},
		{&T{[]int{}}, "[P]", &T{[]int{}}},
		{&T{[]int{1, 2, 3}}, "[P]", &T{[]int{1, 2, 3}}},
		{&T{nil}, "[P]\nS = [1,2]", &T{[]int{1, 2}}},
		{&T{[]int{}}, "[P]\nS = [1,2]", &T{[]int{1, 2}}},
		{&T{[]int{1, 2, 3}}, "[P]\nS = [1,2]", &T{[]int{1, 2}}},
	} {
		var s S
		md, err := Decode(tt.input, &s)
		if err != nil {
			t.Errorf("[%d] Decode error: %s", i, err)
			continue
		}
		if err := md.PrimitiveDecode(s.P, tt.v); err != nil {
			t.Errorf("[%d] PrimitiveDecode error: %s", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.v, tt.want) {
			t.Errorf("[%d] got %#v; want %#v", i, tt.v, tt.want)
		}
	}
}

func TestDecodeDatetime(t *testing.T) {
	// Test here in addition to toml-test to ensure the TZs are correct.
	tz7 := time.FixedZone("", -3600*7)

	for _, tt := range []struct {
		in   string
		want time.Time
	}{
		// Offset datetime
		{"1979-05-27T07:32:00Z", time.Date(1979, 05, 27, 07, 32, 0, 0, time.UTC)},
		{"1979-05-27T07:32:00.999999Z", time.Date(1979, 05, 27, 07, 32, 0, 999999000, time.UTC)},
		{"1979-05-27T00:32:00-07:00", time.Date(1979, 05, 27, 00, 32, 0, 0, tz7)},
		{"1979-05-27T00:32:00.999999-07:00", time.Date(1979, 05, 27, 00, 32, 0, 999999000, tz7)},
		{"1979-05-27T00:32:00.24-07:00", time.Date(1979, 05, 27, 00, 32, 0, 240000000, tz7)},
		{"1979-05-27 07:32:00Z", time.Date(1979, 05, 27, 07, 32, 0, 0, time.UTC)},
		{"1979-05-27t07:32:00z", time.Date(1979, 05, 27, 07, 32, 0, 0, time.UTC)},

		// Make sure the space between the datetime and "#" isn't lexed.
		{"1979-05-27T07:32:12-07:00  # c", time.Date(1979, 05, 27, 07, 32, 12, 0, tz7)},

		// Local times.
		{"1979-05-27T07:32:00", time.Date(1979, 05, 27, 07, 32, 0, 0, internal.LocalDatetime)},
		{"1979-05-27T07:32:00.999999", time.Date(1979, 05, 27, 07, 32, 0, 999999000, internal.LocalDatetime)},
		{"1979-05-27T07:32:00.25", time.Date(1979, 05, 27, 07, 32, 0, 250000000, internal.LocalDatetime)},
		{"1979-05-27", time.Date(1979, 05, 27, 0, 0, 0, 0, internal.LocalDate)},
		{"07:32:00", time.Date(0, 1, 1, 07, 32, 0, 0, internal.LocalTime)},
		{"07:32:00.999999", time.Date(0, 1, 1, 07, 32, 0, 999999000, internal.LocalTime)},
	} {
		t.Run(tt.in, func(t *testing.T) {
			var x struct{ D time.Time }
			input := "d = " + tt.in
			if _, err := Decode(input, &x); err != nil {
				t.Fatalf("got error: %s", err)
			}

			if h, w := x.D.Format(time.RFC3339Nano), tt.want.Format(time.RFC3339Nano); h != w {
				t.Errorf("\nhave: %s\nwant: %s", h, w)
			}
		})
	}
}

func TestMetaDotConflict(t *testing.T) {
	var m map[string]interface{}
	meta, err := Decode(`
		"a.b" = "str"
		a.b   = 1
		""    = 2
	`, &m)
	if err != nil {
		t.Fatal(err)
	}

	want := `"a.b"=String; a.b=Integer; ""=Integer`
	have := ""
	for i, k := range meta.Keys() {
		if i > 0 {
			have += "; "
		}
		have += k.String() + "=" + meta.Type(k...)
	}
	if have != want {
		t.Errorf("\nhave: %s\nwant: %s", have, want)
	}
}

// errorContains checks if the error message in have contains the text in
// want.
//
// This is safe when have is nil. Use an empty string for want if you want to
// test that err is nil.
func errorContains(have error, want string) bool {
	if have == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(have.Error(), want)
}
