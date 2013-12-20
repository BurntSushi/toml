package toml

// TODO: Build a decent encoder.
// Interestingly, this isn't as trivial as recursing down the type of the
// value given and outputting the corresponding TOML. In particular, multiple
// TOML types (especially if tuples are added) can map to a single Go type, so
// that the reverse correspondence isn't clear.
//
// One possible avenue is to choose a reasonable default (like structs map
// to hashes), but allow the user to override with struct tags. But this seems
// like a mess.
//
// The other possibility is to scrap an encoder altogether. After all, TOML
// is a configuration file format, and not a data exchange format.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrArrayMixedElementTypes = errors.New("can't encode array with mixed element types")
	ErrArrayNilElement        = errors.New("can't encode array with nil element")
)

type encoder struct {
	// A single indentation level. By default it is two spaces.
	Indent string

	w *bufio.Writer
}

func newEncoder(w io.Writer) *encoder {
	return &encoder{
		w:      bufio.NewWriter(w),
		Indent: "  ",
	}
}

func (enc *encoder) Encode(v interface{}) error {
	rv := eindirect(reflect.ValueOf(v))
	if err := enc.encode(Key([]string{}), rv); err != nil {
		return err
	}
	return enc.w.Flush()
}

func (enc *encoder) encode(key Key, rv reflect.Value) error {
	k := rv.Kind()
	switch k {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Array, reflect.Slice, reflect.String:
		err := enc.eKeyEq(key)
		if err != nil {
			return err
		}
		return enc.eElement(rv)
	case reflect.Struct:
		return enc.eStruct(key, rv)
	}
	return e("Unsupported type for key '%s': %s", key, k)
}

// eElement encodes any value that can be an array element (primitives and arrays).
func (enc *encoder) eElement(rv reflect.Value) error {
	var err error
	k := rv.Kind()
	switch k {
	case reflect.Bool:
		_, err = io.WriteString(enc.w, strconv.FormatBool(rv.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		_, err = io.WriteString(enc.w, strconv.FormatInt(rv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		_, err = io.WriteString(enc.w, strconv.FormatUint(rv.Uint(), 10))
	case reflect.Float32:
		_, err = io.WriteString(enc.w, strconv.FormatFloat(rv.Float(), 'f', -1, 32))
	case reflect.Float64:
		_, err = io.WriteString(enc.w, strconv.FormatFloat(rv.Float(), 'f', -1, 64))
	case reflect.Array, reflect.Slice:
		return enc.eArrayOrSlice(rv)
	case reflect.Interface:
		return enc.eElement(rv.Elem())
	case reflect.String:
		s := rv.String()
		s = strings.NewReplacer(
			"\t", "\\t",
			"\n", "\\n",
			"\r", "\\r",
			"\"", "\\\"",
			"\\", "\\\\",
		).Replace(s)
		s = "\"" + s + "\""
		_, err = io.WriteString(enc.w, s)
	default:
		return e("Unexpected primitive type: %s", k)
	}
	return err
}

func (enc *encoder) eArrayOrSlice(rv reflect.Value) error {
	if _, err := enc.w.Write([]byte{'['}); err != nil {
		return err
	}

	length := rv.Len()
	if length > 0 {
		arrayElemType, isNil := tomlTypeName(rv.Index(0))
		if isNil {
			return ErrArrayNilElement
		}

		for i := 0; i < length; i++ {
			elem := rv.Index(i)

			// Ensure that the array's elements each have the same TOML type.
			elemType, isNil := tomlTypeName(elem)
			if isNil {
				return ErrArrayNilElement
			}
			if elemType != arrayElemType {
				return ErrArrayMixedElementTypes
			}

			if err := enc.eElement(elem); err != nil {
				return err
			}
			if i != length-1 {
				if _, err := enc.w.Write([]byte(", ")); err != nil {
					return err
				}
			}
		}
	}

	if _, err := enc.w.Write([]byte{']'}); err != nil {
		return err
	}
	return nil
}

func (enc *encoder) eStruct(key Key, rv reflect.Value) error {
	if len(key) > 0 {
		_, err := fmt.Fprintf(enc.w, "[%s]\n", key[len(key)-1])
		if err != nil {
			return err
		}
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sft := rt.Field(i)
		sf := rv.Field(i)
		if isNil(sf) {
			// Don't write anything for nil fields.
			continue
		}
		if err := enc.encode(key.add(sft.Name), sf); err != nil {
			return err
		}

		if i != rt.NumField()-1 {
			if _, err := enc.w.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}
	return nil
}

// tomlTypeName returns the TOML type name of the Go value's type. It is used to
// determine whether the types of array elements are mixed (which is forbidden).
// If the Go value is nil, then it is illegal for it to be an array element, and
// valueIsNil is returned as true.
func tomlTypeName(rv reflect.Value) (typeName string, valueIsNil bool) {
	if isNil(rv) {
		return "", true
	}
	k := rv.Kind()
	switch k {
	case reflect.Bool:
		return "bool", false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer", false
	case reflect.Float32, reflect.Float64:
		return "float", false
	case reflect.Array, reflect.Slice:
		return "array", false
	case reflect.Interface:
		return tomlTypeName(rv.Elem())
	case reflect.String:
		return "string", false
	default:
		panic("unexpected reflect.Kind: " + k.String())
	}
}

func isNil(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func (enc *encoder) eKeyEq(key Key) error {
	_, err := io.WriteString(enc.w, strings.Repeat(enc.Indent, len(key)-1))
	if err != nil {
		return err
	}
	_, err = io.WriteString(enc.w, key[len(key)-1]+" = ")
	if err != nil {
		return err
	}
	return nil
}

func eindirect(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Ptr {
		return v
	}
	return eindirect(reflect.Indirect(v))
}
