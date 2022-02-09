package toml_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"strconv"
	"strings"
	"testing"
)

// Test for hotfix-341
func TestCustomEncode(t *testing.T) {

	var enum = Enum(OtherValue)

	outer := Outer{
		String:  &InnerString{value: "value"},
		Int:     &InnerInt{value: 10},
		Bool:    &InnerBool{value: true},
		Enum:    &enum,
		ArrayS:  &InnerArrayString{value: []string{"text1", "text2"}},
		ArrayI:  &InnerArrayInt{value: []int64{5, 7, 3}},
	}

	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(outer)
	if err != nil {
		t.Errorf("Encode failed: %s", err)
	}

	have := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("String = \"value\"\nInt = 10\nBool = true\nEnum = \"OTHER_VALUE\"\nArrayS = [\"text1\", \"text2\"]\nArrayI = [5, 7, 3]\n")
	if want != have {
		t.Errorf("\nhave:\n%s\nwant:\n%s\n", have, want)
	}
}

// Test for hotfix-341
func TestCustomDecode(t *testing.T) {

	const testToml = "Bool = true\nString = \"test\"\nInt = 10\nEnum = \"OTHER_VALUE\"\nArrayS = [\"text1\", \"text2\"]\nArrayI = [5, 7, 3]"

	outer := Outer{}
	_, err := toml.Decode(testToml, &outer)

	if err != nil {
		t.Fatal(fmt.Sprintf("Decode failed: %s", err))
	}
	if outer.String.value != "test" {
		t.Errorf("\nhave:\n%s\nwant:\n%s\n", outer.String.value, "test")
	}
	if outer.Bool.value != true {
		t.Errorf("\nhave:\n%v\nwant:\n%v\n", outer.Bool.value, true)
	}
	if outer.Int.value != 10 {
		t.Errorf("\nhave:\n%v\nwant:\n%v\n", outer.Int.value, 10)
	}

	if *outer.Enum != OtherValue {
		t.Errorf("\nhave:\n%v\nwant:\n%v\n", outer.Enum, OtherValue)
	}
	if fmt.Sprint(outer.ArrayS.value) != fmt.Sprint([]string{"text1", "text2"}) {
		t.Errorf("\nhave:\n%v\nwant:\n%v\n", outer.ArrayS.value, []string{"text1", "text2"})
	}
	if fmt.Sprint(outer.ArrayI.value) != fmt.Sprint([]int64{5, 7, 3}) {
		t.Errorf("\nhave:\n%v\nwant:\n%v\n", outer.ArrayI.value, []int64{5, 7, 3})
	}
}

/* Implementing MarshalTOML and UnmarshalTOML structs
   An useful use could be to map a TOML value to an internal value, like emuns.
*/

type Enum int

const (
	NoValue Enum = iota
	SomeValue
	OtherValue
)

func (e *Enum) Value() string {
	switch *e {
	case SomeValue:
		return "SOME_VALUE"
	case OtherValue:
		return "OTHER_VALUE"
	case NoValue:
		return ""
	}
	return ""
}

func (e *Enum) MarshalTOML() ([]byte, error) {
	return []byte("\"" + e.Value() + "\""), nil
}

func (e *Enum) UnmarshalTOML(value interface{}) error {
	sValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("value %v is not a string type", value)
	}
	for _, enum := range []Enum{NoValue, SomeValue, OtherValue} {
		if enum.Value() == sValue {
			*e = enum
			return nil
		}
	}
	return errors.New("invalid enum value")
}

type InnerString struct {
	value string
}

func (s *InnerString) MarshalTOML() ([]byte, error) {
	return []byte("\"" + s.value + "\""), nil
}
func (s *InnerString) UnmarshalTOML(value interface{}) error {
	sValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("value %v is not a string type", value)
	}
	s.value = sValue
	return nil
}

type InnerInt struct {
	value int
}

func (i *InnerInt) MarshalTOML() ([]byte, error) {
	return []byte(strconv.Itoa(i.value)), nil
}
func (i *InnerInt) UnmarshalTOML(value interface{}) error {
	iValue, ok := value.(int64)
	if !ok {
		return fmt.Errorf("value %v is not a int type", value)
	}
	i.value = int(iValue)
	return nil
}

type InnerBool struct {
	value bool
}

func (b *InnerBool) MarshalTOML() ([]byte, error) {
	return []byte(strconv.FormatBool(b.value)), nil
}
func (b *InnerBool) UnmarshalTOML(value interface{}) error {
	bValue, ok := value.(bool)
	if !ok {
		return fmt.Errorf("value %v is not a bool type", value)
	}
	b.value = bValue
	return nil
}


type InnerArrayString struct {
	value []string
}

func (as *InnerArrayString) MarshalTOML() ([]byte, error) {
	return []byte("[\"" + strings.Join(as.value, "\", \"") + "\"]"), nil
}

func (as *InnerArrayString) UnmarshalTOML(value interface{}) error {
	if value != nil {
		asValue, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("value %v is not a [] type", value)
		}
		as.value = []string{}
		for _, value := range asValue {
			as.value = append(as.value, value.(string))
		}
	}
	return nil
}

type InnerArrayInt struct {
	value []int64
}

func (ai *InnerArrayInt) MarshalTOML() ([]byte, error) {
	strArr := []string{}
	for _, intV := range ai.value {
		strArr = append(strArr, strconv.FormatInt(intV, 10))
	}
	return []byte("[" + strings.Join(strArr, ", ") + "]"), nil
}

func (ai *InnerArrayInt) UnmarshalTOML(value interface{}) error {
	if value != nil {
		asValue, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("value %v is not a [] type", value)
		}
		ai.value = []int64{}
		for _, value := range asValue {
			ai.value = append(ai.value, value.(int64))
		}
	}
	return nil
}

type Outer struct {
	String  *InnerString
	Int     *InnerInt
	Bool    *InnerBool
	Enum    *Enum
	ArrayS  *InnerArrayString
	ArrayI  *InnerArrayInt
}
