package toml

import (
	"bufio"
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml/internal"
)

type tomlEncodeError struct{ error }

var (
	errArrayNilElement = errors.New("toml: cannot encode array with nil element")
	errNonString       = errors.New("toml: cannot encode a map with non-string key type")
	errAnonNonStruct   = errors.New("toml: cannot encode an anonymous field that is not a struct")
	errNoKey           = errors.New("toml: top-level values must be Go maps or structs")
	errAnything        = errors.New("") // used in testing
)

var dblQuotedReplacer = strings.NewReplacer(
	"\"", "\\\"",
	"\\", "\\\\",
	"\x00", `\u0000`,
	"\x01", `\u0001`,
	"\x02", `\u0002`,
	"\x03", `\u0003`,
	"\x04", `\u0004`,
	"\x05", `\u0005`,
	"\x06", `\u0006`,
	"\x07", `\u0007`,
	"\b", `\b`,
	"\t", `\t`,
	"\n", `\n`,
	"\x0b", `\u000b`,
	"\f", `\f`,
	"\r", `\r`,
	"\x0e", `\u000e`,
	"\x0f", `\u000f`,
	"\x10", `\u0010`,
	"\x11", `\u0011`,
	"\x12", `\u0012`,
	"\x13", `\u0013`,
	"\x14", `\u0014`,
	"\x15", `\u0015`,
	"\x16", `\u0016`,
	"\x17", `\u0017`,
	"\x18", `\u0018`,
	"\x19", `\u0019`,
	"\x1a", `\u001a`,
	"\x1b", `\u001b`,
	"\x1c", `\u001c`,
	"\x1d", `\u001d`,
	"\x1e", `\u001e`,
	"\x1f", `\u001f`,
	"\x7f", `\u007f`,
)

// Marshaler is the interface implemented by types that can marshal themselves
// into valid TOML.
type Marshaler interface {
	MarshalTOML() ([]byte, error)
}

// Encoder encodes a Go to a TOML document.
//
// The mapping between Go values and TOML values should be precisely the same as
// for the Decode* functions.
//
// The toml.Marshaler and encoder.TextMarshaler interfaces are supported to
// encoding the value as custom TOML.
//
// If you want to write arbitrary binary data then you will need to use
// something like base64 since TOML does not have any binary types.
//
// When encoding TOML hashes (Go maps or structs), keys without any sub-hashes
// are encoded first.
//
// Go maps will be sorted alphabetically by key for deterministic output.
//
// Encoding Go values without a corresponding TOML representation will return an
// error. Examples of this includes maps with non-string keys, slices with nil
// elements, embedded non-struct types, and nested slices containing maps or
// structs. (e.g. [][]map[string]string is not allowed but []map[string]string
// is okay, as is []map[string][]string).
//
// NOTE: only exported keys are encoded due to the use of reflection. Unexported
// keys are silently discarded.
type Encoder struct {
	// String to use for a single indentation level; default is two spaces.
	Indent string

	// TODO(v2): Ident should be a function so we can do:
	//
	//   NewEncoder(os.Stdout).SetIndent("prefix", "indent").MetaData(meta).Encode()
	//
	// Prefix is also useful to have.

	w          *bufio.Writer
	hasWritten bool // written any output to w yet?
	wroteNL    int  // How many newlines do we have in a row?
	meta       *MetaData
}

// NewEncoder create a new Encoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:      bufio.NewWriter(w),
		Indent: "  ",
	}
}

// MetaData sets the metadata for this encoder.
//
// This can be used to control the formatting; see the documentation of MetaData
// for more details.
//
// XXX: Rename to SetMeta()
func (enc *Encoder) MetaData(m MetaData) *Encoder {
	enc.meta = &m
	return enc
}

// Encode writes a TOML representation of the Go value to the Encoder's writer.
//
// An error is returned if the value given cannot be encoded to a valid TOML
// document.
func (enc *Encoder) Encode(v interface{}) error {
	rv := eindirect(reflect.ValueOf(v))
	if err := enc.safeEncode(Key([]string{}), rv); err != nil {
		return err
	}
	return enc.w.Flush()
}

func (enc *Encoder) safeEncode(key Key, rv reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if terr, ok := r.(tomlEncodeError); ok {
				err = terr.error
				return
			}
			panic(r)
		}
	}()
	enc.encode(key, rv)
	return nil
}

// Newline rules:

// nocomment = "value"
// no_bl = 1
//
// # With comment: has blank line before the comment, and one after the key.
// with_c = 2
//
// asd = 1
// qwe = 2  # After comment: no extra newline
// zxc = 3
//
// [tbl]  # Always has newline before it.key
//
// # With comment
// [tbl2]
//   key1 = 123
//
//
func (enc *Encoder) encode(key Key, rv reflect.Value) {
	extraNL := false
	if enc.meta != nil && enc.meta.comments != nil {
		comments := enc.meta.comments[key.String()]
		for _, c := range comments {
			if c.where == commentDoc {
				extraNL = true
				enc.w.WriteString("# ")
				enc.w.WriteString(strings.ReplaceAll(c.text, "\n", "\n# "))
				enc.newline(1)
			}
		}
	}

	// Special case: time needs to be in ISO8601 format.
	//
	// Special case: if we can marshal the type to text, then we used that. This
	// prevents the encoder for handling these types as generic structs (or
	// whatever the underlying type of a TextMarshaler is).
	switch t := rv.Interface().(type) {
	case time.Time, encoding.TextMarshaler, Marshaler:
		enc.writeKeyValue(key, rv, false)
	// TODO: #76 would make this superfluous after implemented.
	// TODO: remove in v2
	case Primitive:
		enc.encode(key, reflect.ValueOf(t.undecoded))
	default:

		k := rv.Kind()
		switch k {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
			reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
			reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.String, reflect.Bool:
			enc.writeKeyValue(key, rv, false)
		case reflect.Array, reflect.Slice:
			if typeEqual(ArrayTable{}, tomlTypeOfGo(rv)) {
				enc.eArrayOfTables(key, rv)
			} else {
				enc.writeKeyValue(key, rv, false)
			}
		case reflect.Interface:
			if rv.IsNil() {
				return
			}
			enc.encode(key, rv.Elem())
		case reflect.Map:
			if rv.IsNil() {
				return
			}
			enc.eTable(key, rv)
		case reflect.Ptr:
			if rv.IsNil() {
				return
			}
			enc.encode(key, rv.Elem())
		case reflect.Struct:
			enc.eTable(key, rv)
		default:
			encPanic(fmt.Errorf("unsupported type for key '%s': %s", key, k))
		}
	}

	// Write comments after the key.
	if enc.meta != nil && enc.meta.comments != nil {
		comments := enc.meta.comments[key.String()]
		for _, c := range comments {
			if c.where == commentComment {
				enc.w.WriteString("  # ")
				enc.w.WriteString(strings.ReplaceAll(c.text, "\n", "\n# "))
				enc.newline(1)
			}
		}
	}

	enc.newline(1)
	if extraNL {
		enc.newline(1)
	}
}

func (enc *Encoder) writeInt(typ tomlType, v uint64) {
	var (
		iTyp = asInt(typ)
		base = int(iTyp.Base)
	)
	switch iTyp.Base {
	case 0:
		base = 10
	case 2:
		enc.wf("0b")
	case 8:
		enc.wf("0o")
	case 16:
		enc.wf("0x")
	}

	n := strconv.FormatUint(uint64(v), base)
	if base != 10 && iTyp.Width > 0 && len(n) < int(iTyp.Width) {
		enc.wf(strings.Repeat("0", int(iTyp.Width)-len(n)))
	}
	enc.wf(n)
}

// eElement encodes any value that can be an array element.
func (enc *Encoder) eElement(rv reflect.Value, typ tomlType) {
	//fmt.Printf("ENC %T -> %s -> %[1]v\n", rv.Interface(), typ)

	switch v := rv.Interface().(type) {
	case time.Time: // Using TextMarshaler adds extra quotes, which we don't want.
		format := ""
		switch asDatetime(typ).Format {
		case 0: // Undefined, check for special TZ.
			format = time.RFC3339Nano
			switch v.Location() {
			case internal.LocalDatetime:
				format = "2006-01-02T15:04:05.999999999"
			case internal.LocalDate:
				format = "2006-01-02"
			case internal.LocalTime:
				format = "15:04:05.999999999"
			}

		case DatetimeFormatFull:
			format = time.RFC3339Nano
		case DatetimeFormatLocal:
			format = "2006-01-02T15:04:05.999999999"
		case DatetimeFormatDate:
			format = "2006-01-02"
		case DatetimeFormatTime:
			format = "15:04:05.999999999"
		default:
			encPanic(fmt.Errorf("Invalid datetime format: %v", asDatetime(typ).Format))
		}

		//fmt.Printf("ENC %T -> %s -> %[1]v\n", rv.Interface(), typ)
		//fmt.Println("XXX", asDatetime(typ).Format)
		if format != time.RFC3339Nano {
			//v = v.In(time.UTC)
		}

		//switch v.Location() {
		//default:
		enc.wf(v.Format(format))
		//case internal.LocalDatetime, internal.LocalDate, internal.LocalTime:
		//	enc.wf(v.In(time.UTC).Format(format))
		//}
		return
	case Marshaler:
		s, err := v.MarshalTOML()
		if err != nil {
			encPanic(err)
		}
		enc.writeQuoted(string(s), asString(typ))
		return
	case encoding.TextMarshaler:
		s, err := v.MarshalText()
		if err != nil {
			encPanic(err)
		}
		enc.writeQuoted(string(s), asString(typ))
		return
	}

	switch rv.Kind() {
	case reflect.Bool:
		enc.wf(strconv.FormatBool(rv.Bool()))
	case reflect.String:
		enc.writeQuoted(rv.String(), asString(typ))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := rv.Int()
		if v < 0 { // Make sure sign is before "0x".
			enc.wf("-")
			v = -v
		}
		enc.writeInt(typ, uint64(v))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		enc.writeInt(typ, rv.Uint())

	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if math.IsNaN(f) {
			enc.wf("nan")
		} else if math.IsInf(f, 0) {
			enc.wf("%cinf", map[bool]byte{true: '-', false: '+'}[math.Signbit(f)])
		} else {
			n := 64
			if rv.Kind() == reflect.Float32 {
				n = 32
			}
			if asFloat(typ).Exponent {
				enc.wf(strconv.FormatFloat(f, 'e', -1, n))
			} else {
				enc.wf(floatAddDecimal(strconv.FormatFloat(f, 'f', -1, n)))
			}
		}

	case reflect.Array, reflect.Slice:
		enc.eArrayOrSliceElement(rv)
	case reflect.Struct:
		enc.eStruct(nil, rv, true)
	case reflect.Map:
		enc.eMap(nil, rv, true)
	case reflect.Interface:
		enc.eElement(rv.Elem(), typ)
	default:
		encPanic(fmt.Errorf("unexpected primitive type: %T", rv.Interface()))
	}
}

// By the TOML spec, all floats must have a decimal with at least one number on
// either side.
func floatAddDecimal(fstr string) string {
	if !strings.Contains(fstr, ".") {
		return fstr + ".0"
	}
	return fstr
}

func (enc *Encoder) writeQuoted(s string, typ String) {
	if typ.Literal {
		if typ.Multiline {
			enc.wf("'''%s'''\n", s)
		} else {
			enc.wf(`'%s'`, s)
		}
	} else {
		if typ.Multiline {
			enc.wf(`"""%s"""`+"\n",
				strings.ReplaceAll(dblQuotedReplacer.Replace(s), "\\n", "\n"))
		} else {
			enc.wf(`"%s"`, dblQuotedReplacer.Replace(s))
		}
	}
}

func (enc *Encoder) eArrayOrSliceElement(rv reflect.Value) {
	length := rv.Len()
	enc.wf("[")
	for i := 0; i < length; i++ {
		elem := rv.Index(i)
		enc.eElement(elem, nil) // XXX: add type
		if i != length-1 {
			enc.wf(", ")
		}
	}
	enc.wf("]")
}

func (enc *Encoder) eArrayOfTables(key Key, rv reflect.Value) {
	if len(key) == 0 {
		encPanic(errNoKey)
	}
	for i := 0; i < rv.Len(); i++ {
		trv := rv.Index(i)
		if isNil(trv) {
			continue
		}

		enc.newline(2)
		enc.wf("%s[[%s]]", enc.indentStr(key), key.maybeQuotedAll())
		enc.newline(1)
		enc.eMapOrStruct(key, trv, false)
	}
}

func (enc *Encoder) eTable(key Key, rv reflect.Value) {
	if len(key) == 1 { // Output an extra newline between top-level tables.
		enc.newline(2)
	}
	if len(key) > 0 {
		enc.wf("%s[%s]", enc.indentStr(key), key.maybeQuotedAll())
		enc.newline(1)
	}
	enc.eMapOrStruct(key, rv, false)
}

func (enc *Encoder) eMapOrStruct(key Key, rv reflect.Value, inline bool) {
	switch rv := eindirect(rv); rv.Kind() {
	case reflect.Map:
		enc.eMap(key, rv, inline)
	case reflect.Struct:
		enc.eStruct(key, rv, inline)
	default:
		// Should never happen?
		panic("eTable: unhandled reflect.Value Kind: " + rv.Kind().String())
	}
}

func (enc *Encoder) eMap(key Key, rv reflect.Value, inline bool) {
	rt := rv.Type()
	if rt.Key().Kind() != reflect.String {
		encPanic(errNonString)
	}

	// Sort keys so that we have deterministic output. And write keys directly
	// underneath this key first, before writing sub-structs or sub-maps.
	var mapKeysDirect, mapKeysSub []string
	for _, mapKey := range rv.MapKeys() {
		k := mapKey.String()
		if typeIsTable(tomlTypeOfGo(rv.MapIndex(mapKey))) {
			mapKeysSub = append(mapKeysSub, k)
		} else {
			mapKeysDirect = append(mapKeysDirect, k)
		}
	}

	var writeMapKeys = func(mapKeys []string, trailC bool) {
		sort.Strings(mapKeys)
		for i, mapKey := range mapKeys {
			val := rv.MapIndex(reflect.ValueOf(mapKey))
			if isNil(val) {
				continue
			}

			if inline {
				enc.writeKeyValue(Key{mapKey}, val, true)
				if trailC || i != len(mapKeys)-1 {
					enc.wf(", ")
				}
			} else {
				enc.encode(key.add(mapKey), val)
			}
		}
	}

	if inline {
		enc.wf("{")
	}
	writeMapKeys(mapKeysDirect, len(mapKeysSub) > 0)
	writeMapKeys(mapKeysSub, false)
	if inline {
		enc.wf("}")
	}
}

const is32Bit = (32 << (^uint(0) >> 63)) == 32

func (enc *Encoder) eStruct(key Key, rv reflect.Value, inline bool) {
	// Write keys for fields directly under this key first, because if we write
	// a field that creates a new table then all keys under it will be in that
	// table (not the one we're writing here).
	//
	// Fields is a [][]int: for fieldsDirect this always has one entry (the
	// struct index). For fieldsSub it contains two entries: the parent field
	// index from tv, and the field indexes for the fields of the sub.
	var (
		rt                      = rv.Type()
		fieldsDirect, fieldsSub [][]int
		addFields               func(rt reflect.Type, rv reflect.Value, start []int)
	)
	addFields = func(rt reflect.Type, rv reflect.Value, start []int) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.PkgPath != "" && !f.Anonymous { /// Skip unexported fields.
				continue
			}

			frv := rv.Field(i)

			// Treat anonymous struct fields with tag names as though they are
			// not anonymous, like encoding/json does.
			//
			// Non-struct anonymous fields use the normal encoding logic.
			if f.Anonymous {
				t := f.Type
				switch t.Kind() {
				case reflect.Struct:
					if getOptions(f.Tag).name == "" {
						addFields(t, frv, append(start, f.Index...))
						continue
					}
				case reflect.Ptr:
					if t.Elem().Kind() == reflect.Struct && getOptions(f.Tag).name == "" {
						if !frv.IsNil() {
							addFields(t.Elem(), frv.Elem(), append(start, f.Index...))
						}
						continue
					}
				}
			}

			if typeIsTable(tomlTypeOfGo(frv)) {
				fieldsSub = append(fieldsSub, append(start, f.Index...))
			} else {
				// Copy so it works correct on 32bit archs; not clear why this
				// is needed. See #314, and https://www.reddit.com/r/golang/comments/pnx8v4
				// This also works fine on 64bit, but 32bit archs are somewhat
				// rare and this is a wee bit faster.
				if is32Bit {
					copyStart := make([]int, len(start))
					copy(copyStart, start)
					fieldsDirect = append(fieldsDirect, append(copyStart, f.Index...))
				} else {
					fieldsDirect = append(fieldsDirect, append(start, f.Index...))
				}
			}
		}
	}
	addFields(rt, rv, nil)

	writeFields := func(fields [][]int) {
		for _, fieldIndex := range fields {
			fieldType := rt.FieldByIndex(fieldIndex)
			fieldVal := rv.FieldByIndex(fieldIndex)

			if isNil(fieldVal) { /// Don't write anything for nil fields.
				continue
			}

			opts := getOptions(fieldType.Tag)
			if opts.skip {
				continue
			}
			keyName := fieldType.Name
			if opts.name != "" {
				keyName = opts.name
			}
			if opts.omitempty && isEmpty(fieldVal) {
				continue
			}
			if opts.omitzero && isZero(fieldVal) {
				continue
			}

			if inline {
				enc.writeKeyValue(Key{keyName}, fieldVal, true)
				if fieldIndex[0] != len(fields)-1 {
					enc.wf(", ")
				}
			} else {
				enc.encode(key.add(keyName), fieldVal)
			}
		}
	}

	if inline {
		enc.wf("{")
	}
	writeFields(fieldsDirect)
	writeFields(fieldsSub)
	if inline {
		enc.wf("}")
	}
}

// tomlTypeOfGo returns the TOML type name of the Go value's type.
//
// It is used to determine whether the types of array elements are mixed (which
// is forbidden). If the Go value is nil, then it is illegal for it to be an
// array element, and valueIsNil is returned as true.
//
// The type may be `nil`, which means no concrete TOML type could be found.
func tomlTypeOfGo(rv reflect.Value) tomlType {
	if isNil(rv) || !rv.IsValid() {
		return nil
	}
	switch rv.Kind() {
	case reflect.Bool:
		return Bool{}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64:
		return Int{}
	case reflect.Float32, reflect.Float64:
		return Float{}
	case reflect.Array, reflect.Slice:
		if typeEqual(Table{}, tomlArrayType(rv)) {
			return ArrayTable{}
		}
		return Array{}
	case reflect.Ptr, reflect.Interface:
		return tomlTypeOfGo(rv.Elem())
	case reflect.String:
		return String{}
	case reflect.Map:
		return Table{}
	case reflect.Struct:
		switch rv.Interface().(type) {
		case time.Time:
			return Datetime{}
		case encoding.TextMarshaler:
			return String{}
		default:
			// Someone used a pointer receiver: we can make it work for pointer
			// values.
			if rv.CanAddr() {
				_, ok := rv.Addr().Interface().(encoding.TextMarshaler)
				if ok {
					return String{}
				}
			}
			return Table{}
		}
	default:
		_, ok := rv.Interface().(encoding.TextMarshaler)
		if ok {
			return String{}
		}
		encPanic(errors.New("unsupported type: " + rv.Kind().String()))
		panic("") // Need *some* return value
	}
}

// tomlArrayType returns the element type of a TOML array. The type returned
// may be nil if it cannot be determined (e.g., a nil slice or a zero length
// slize). This function may also panic if it finds a type that cannot be
// expressed in TOML (such as nil elements, heterogeneous arrays or directly
// nested arrays of tables).
func tomlArrayType(rv reflect.Value) tomlType {
	if isNil(rv) || !rv.IsValid() || rv.Len() == 0 {
		return nil
	}

	/// Don't allow nil.
	rvlen := rv.Len()
	for i := 1; i < rvlen; i++ {
		if tomlTypeOfGo(rv.Index(i)) == nil {
			encPanic(errArrayNilElement)
		}
	}

	firstType := tomlTypeOfGo(rv.Index(0))
	if firstType == nil {
		encPanic(errArrayNilElement)
	}
	return firstType
}

type tagOptions struct {
	skip      bool // "-"
	name      string
	omitempty bool
	omitzero  bool
}

func getOptions(tag reflect.StructTag) tagOptions {
	t := tag.Get("toml")
	if t == "-" {
		return tagOptions{skip: true}
	}
	var opts tagOptions
	parts := strings.Split(t, ",")
	opts.name = parts[0]
	for _, s := range parts[1:] {
		switch s {
		case "omitempty":
			opts.omitempty = true
		case "omitzero":
			opts.omitzero = true
		}
	}
	return opts
}

func isZero(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0.0
	}
	return false
}

func isEmpty(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	}
	return false
}

// newline ensures there are n newlines here.
func (enc *Encoder) newline(n int) {
	// Don't write any newlines at the top of the file.
	if !enc.hasWritten {
		return
	}

	w := n - enc.wroteNL
	if w <= 0 {
		return
	}

	enc.wroteNL += w
	//enc.wf(strings.Repeat("\n", w))
	_, err := enc.w.Write(bytes.Repeat([]byte("\n"), w))
	if err != nil {
		encPanic(err)
	}
}

// Write a key/value pair:
//
//   key = <any value>
//
// This is also used for "k = v" in inline tables; so something like this will
// be written in three calls:
//
//     ┌────────────────────┐
//     │      ┌───┐  ┌─────┐│
//     v      v   v  v     vv
//     key = {k = v, k2 = v2}
func (enc *Encoder) writeKeyValue(key Key, val reflect.Value, inline bool) {
	if len(key) == 0 {
		encPanic(errNoKey)
	}
	enc.wf("%s%s = ", enc.indentStr(key), key.maybeQuoted(len(key)-1))

	var typ tomlType
	if enc.meta != nil {
		if t, ok := enc.meta.types[key.String()]; ok {
			typ = t
		}
	}
	enc.eElement(val, typ)
	// if !inline {
	// 	enc.newline()
	// }
}

func (enc *Encoder) wf(format string, v ...interface{}) {
	_, err := fmt.Fprintf(enc.w, format, v...)
	if err != nil {
		encPanic(err)
	}
	enc.wroteNL = 0
	enc.hasWritten = true
}

func (enc *Encoder) indentStr(key Key) string {
	return strings.Repeat(enc.Indent, len(key)-1)
}

func encPanic(err error) {
	panic(tomlEncodeError{err})
}

func eindirect(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return eindirect(v.Elem())
	default:
		return v
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
