package toml

import "encoding"

// DEPRECATED!
//
// Use the identical encoding.TextMarshaler instead. It is defined here to
// support Go 1.1 and older.
type TextMarshaler encoding.TextMarshaler

// DEPRECATED!
//
// Use the identical encoding.TextUnmarshaler instead. It is defined here to
// support Go 1.1 and older.
type TextUnmarshaler encoding.TextUnmarshaler

// DEPRECATED!
//
// Use MetaData.PrimitiveDecode instead.
func PrimitiveDecode(primValue Primitive, v interface{}) error {
	md := MetaData{decoded: make(map[string]bool)}
	return md.unify(primValue.undecoded, rvalue(v))
}
