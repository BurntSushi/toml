package toml

import (
	"encoding"
	"io"
)

// TextMarshaler is an alias for encoding.TextMarshaler.
//
// Deprecated: use encoding.TextMarshaler
type TextMarshaler encoding.TextMarshaler

// TextUnmarshaler is an alias for encoding.TextUnmarshaler.
//
// Deprecated: use encoding.TextUnmarshaler
type TextUnmarshaler encoding.TextUnmarshaler

// DecodeReader is an alias for NewDecoder(r).Decode(v).
//
// Deprecated: use NewDecoder(reader).Decode(&value).
func DecodeReader(r io.Reader, v any) (MetaData, error) { return NewDecoder(r).Decode(v) }

// PrimitiveDecode is an alias for MetaData.PrimitiveDecode().
//
// Deprecated: use MetaData.PrimitiveDecode.
func PrimitiveDecode(primValue Primitive, v any) error {
	md := MetaData{decoded: make(map[string]struct{})}
	return md.unify(primValue.undecoded, rvalue(v))
}

// Primitive is a TOML value that hasn't been decoded into a Go value.
//
// This type can be used for any value, which will cause decoding to be delayed.
// You can use [PrimitiveDecode] to "manually" decode these values.
//
// NOTE: The underlying representation of a `Primitive` value is subject to
// change. Do not rely on it.
//
// NOTE: Primitive values are still parsed, so using them will only avoid the
// overhead of reflection. They can be useful when you don't know the exact type
// of TOML data until runtime.
//
// Deprecated: use Marshaler interface for customer decoding. Or "any"
// parameters for varying types.
type Primitive struct {
	undecoded any
	context   Key
}

// PrimitiveDecode is just like the other Decode* functions, except it decodes a
// TOML value that has already been parsed. Valid primitive values can *only* be
// obtained from values filled by the decoder functions, including this method.
// (i.e., v may contain more [Primitive] values.)
//
// Meta data for primitive values is included in the meta data returned by the
// Decode* functions with one exception: keys returned by the Undecoded method
// will only reflect keys that were decoded. Namely, any keys hidden behind a
// Primitive will be considered undecoded. Executing this method will update the
// undecoded keys in the meta data. (See the example.)
//
// Deprecated: use Marshaler interface for customer decoding. Or "any"
// parameters for varying types.
func (md *MetaData) PrimitiveDecode(primValue Primitive, v any) error {
	md.context = primValue.context
	defer func() { md.context = nil }()
	return md.unify(primValue.undecoded, rvalue(v))
}
