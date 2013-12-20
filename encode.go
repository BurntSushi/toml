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
	"io"
	"reflect"
	"strconv"
	"strings"
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
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
		err := enc.eKeyEq(key)
		if err != nil {
			return err
		}
		return enc.ePrimitive(rv)
	case reflect.Array, reflect.Slice:
		err := enc.eKeyEq(key)
		if err != nil {
			return err
		}
		return enc.eArrayOrSlice(key, rv)
	case reflect.Struct:
		return enc.eStruct(key, rv)
	}
	return e("Unsupported type for key '%s': %s", key, k)
}

func (enc *encoder) ePrimitive(rv reflect.Value) error {
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

func (enc *encoder) eArrayOrSlice(key Key, rv reflect.Value) error {
	if _, err := enc.w.Write([]byte{'['}); err != nil {
		return err
	}

	length := rv.Len()
	for i := 0; i < length; i++ {
		if err := enc.ePrimitive(rv.Index(i)); err != nil {
			return err
		}
		if i != length-1 {
			if _, err := enc.w.Write([]byte(", ")); err != nil {
				return err
			}
		}
	}

	if _, err := enc.w.Write([]byte{']'}); err != nil {
		return err
	}
	return nil
}

func (enc *encoder) eStruct(key Key, rv reflect.Value) error {
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
		if _, err := enc.w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
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
