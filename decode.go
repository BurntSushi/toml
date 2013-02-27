package toml

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"time"
)

var e = fmt.Errorf

// Decode will decode the contents of `data` in TOML format into a pointer
// `v`.
//
// TOML hashes correspond to Go structs or maps. (Dealer's choice. They can be
// used interchangeably.)
//
// TOML datetimes correspond to Go `time.Time` values.
//
// All other TOML types (float, string, int, bool and array) correspond
// to the obvious Go types.
//
// TOML keys can map to either keys in a Go map or field names in a Go
// struct. The special `toml` struct tag may be used to map TOML keys to
// struct fields that don't match the key name exactly. (See the example.)
//
// The mapping between TOML values and Go values is loose. That is, there
// may exist TOML values that cannot be placed into your representation, and
// there may be parts of your representation that do not correspond to
// TOML values.
//
// This decoder will not handle cyclic types. If a cyclic type is passed,
// `Decode` will not terminate.
func Decode(data string, v interface{}) error {
	mapping, err := parse(data)
	if err != nil {
		return err
	}
	return unify(mapping, rvalue(v))
}

// DecodeFile is just like Decode, except it will automatically read the
// contents of the file at `fpath` and decode it for you.
func DecodeFile(fpath string, v interface{}) error {
	bs, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	return Decode(string(bs), v)
}

// DecodeReader is just like Decode, except it will consume all bytes
// from the reader and decode it for you.
func DecodeReader(r io.Reader, v interface{}) error {
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return Decode(string(bs), v)
}

// unify performs a sort of type unification based on the structure of `rv`,
// which is the client representation.
//
// Any type mismatch produces an error. Finding a type that we don't know
// how to handle produces an unsupported type error.
func unify(data interface{}, rv reflect.Value) error {
	// Special case. Go's `time.Time` is a struct, which we don't want
	// to confuse with a user struct.
	if rv.Type().AssignableTo(rvalue(time.Time{}).Type()) {
		return unifyDatetime(data, rv)
	}

	switch rv.Kind() {
	case reflect.Struct:
		return unifyStruct(data, rv)
	case reflect.Map:
		return unifyMap(data, rv)
	case reflect.Slice:
		return unifySlice(data, rv)
	case reflect.String:
		return unifyString(data, rv)
	case reflect.Float64:
		return unifyFloat64(data, rv)
	case reflect.Int:
		return unifyInt(data, rv)
	case reflect.Bool:
		return unifyBool(data, rv)
	case reflect.Interface:
		// we only support empty interfaces.
		if rv.NumMethod() > 0 {
			e("Unsupported type '%s'.", rv.Kind())
		}
		return unifyAnything(data, rv)
	}
	return e("Unsupported type '%s'.", rv.Kind())
}

func unifyStruct(mapping interface{}, rv reflect.Value) error {
	rt := rv.Type()
	tmap, ok := mapping.(map[string]interface{})
	if !ok {
		return mismatch(rv, "map", mapping)
	}
	for i := 0; i < rt.NumField(); i++ {
		// A little tricky. We want to use the special `toml` name in the
		// struct tag if it exists. In particular, we need to make sure that
		// this struct field is in the current map before trying to unify it.
		sft := rt.Field(i)
		kname := sft.Tag.Get("toml")
		if len(kname) == 0 {
			kname = sft.Name
		}
		if datum, ok := tmap[kname]; ok {
			sf := indirect(rv.Field(i))

			// Don't try to mess with unexported types and other such things.
			if sf.CanSet() {
				if err := unify(datum, sf); err != nil {
					return e("Type mismatch for '%s.%s': %s",
						rt.String(), sft.Name, err)
				}
			} else if len(sft.Tag.Get("toml")) > 0 {
				// Bad user! No soup for you!
				return e("Field '%s.%s' is unexported, and therefore cannot "+
					"be loaded with reflection.", rt.String(), sft.Name)
			}
		}
	}
	return nil
}

func unifyMap(mapping interface{}, rv reflect.Value) error {
	tmap, ok := mapping.(map[string]interface{})
	if !ok {
		return badtype("map", mapping)
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}
	for k, v := range tmap {
		rvkey := indirect(reflect.New(rv.Type().Key()))
		rvval := indirect(reflect.New(rv.Type().Elem()))
		if err := unify(v, rvval); err != nil {
			return err
		}

		rvkey.SetString(k)
		rv.SetMapIndex(rvkey, rvval)
	}
	return nil
}

func unifySlice(data interface{}, rv reflect.Value) error {
	slice, ok := data.([]interface{})
	if !ok {
		return badtype("slice", data)
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeSlice(rv.Type(), len(slice), len(slice)))
	}
	for i, v := range slice {
		sliceval := indirect(rv.Index(i))
		if err := unify(v, sliceval); err != nil {
			return err
		}
	}
	return nil
}

func unifyDatetime(data interface{}, rv reflect.Value) error {
	if _, ok := data.(time.Time); ok {
		rv.Set(reflect.ValueOf(data))
		return nil
	}
	return badtype("time.Time", data)
}

func unifyString(data interface{}, rv reflect.Value) error {
	if s, ok := data.(string); ok {
		rv.SetString(s)
		return nil
	}
	return badtype("string", data)
}

func unifyFloat64(data interface{}, rv reflect.Value) error {
	if num, ok := data.(float64); ok {
		rv.SetFloat(num)
		return nil
	}
	return badtype("float", data)
}

func unifyInt(data interface{}, rv reflect.Value) error {
	if num, ok := data.(int64); ok {
		rv.SetInt(int64(num))
		return nil
	}
	return badtype("integer", data)
}

func unifyBool(data interface{}, rv reflect.Value) error {
	if b, ok := data.(bool); ok {
		rv.SetBool(b)
		return nil
	}
	return badtype("integer", data)
}

func unifyAnything(data interface{}, rv reflect.Value) error {
	// too awesome to fail
	rv.Set(reflect.ValueOf(data))
	return nil
}

// rvalue returns a reflect.Value of `v`. All pointers are resolved.
func rvalue(v interface{}) reflect.Value {
	return indirect(reflect.ValueOf(v))
}

// indirect returns the value pointed to by a pointer.
// Pointers are followed until the value is not a pointer.
// New values are allocated for each nil pointer.
func indirect(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Ptr {
		return v
	}
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return indirect(reflect.Indirect(v))
}

func tstring(rv reflect.Value) string {
	return rv.Type().String()
}

func badtype(expected string, data interface{}) error {
	return e("Expected %s but found '%T'.", expected, data)
}

func mismatch(user reflect.Value, expected string, data interface{}) error {
	return e("Type mismatch for %s. Expected %s but found '%T'.",
		tstring(user), expected, data)
}
