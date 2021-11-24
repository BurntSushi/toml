package toml

import (
	"fmt"
	"strings"
)

// MetaData allows access to meta information about TOML.
//
// It allows determining whether a key has been defined, the TOML type of a
// key, and how it's formatted. It also records comments in the TOML file.
type MetaData struct {
	mapping  map[string]interface{}
	types    map[string]tomlType  // TOML types.
	keys     []Key                // List of defined keys.
	decoded  map[string]bool      // Decoded keys.
	context  Key                  // Used only during decoding.
	comments map[string][]comment // Record comments.
}

const (
	_              = iota
	commentDoc     // Above the key.
	commentComment // "Inline" after the key.
)

type comment struct {
	where int
	text  string
}

func NewMetaData() MetaData {
	return MetaData{}
}

type (
	Doc     string
	Comment string
)

func (enc *MetaData) Key(key string, args ...interface{}) *MetaData {
	for _, a := range args {
		switch aa := a.(type) {
		default:
			panic(fmt.Sprintf("toml.MetaData.Key: unsupported type: %T", a))
		case tomlType:
			enc.SetType(key, aa)
		case Doc:
			enc.Doc(key, string(aa))
		case Comment:
			enc.Comment(key, string(aa))
		}
	}
	return enc
}

func (enc *MetaData) SetType(key string, t tomlType) *MetaData {
	enc.types[key] = t
	return enc
}

func (enc *MetaData) Doc(key string, doc string) *MetaData {
	if enc.comments == nil {
		enc.comments = make(map[string][]comment)
	}
	enc.comments[key] = append(enc.comments[key], comment{where: commentDoc, text: doc})
	return enc
}

func (enc *MetaData) Comment(key string, doc string) *MetaData {
	if enc.comments == nil {
		enc.comments = make(map[string][]comment)
	}
	enc.comments[key] = append(enc.comments[key], comment{where: commentComment, text: doc})
	return enc
}

// IsDefined reports if the key exists in the TOML data.
//
// The key should be specified hierarchically, for example to access the TOML
// key "a.b.c" you would use:
//
//	IsDefined("a", "b", "c")
//
// IsDefined will return false if an empty key given. Keys are case sensitive.
func (md *MetaData) IsDefined(key ...string) bool {
	if len(key) == 0 {
		return false
	}

	var hash map[string]interface{}
	var ok bool
	var hashOrVal interface{} = md.mapping
	for _, k := range key {
		if hash, ok = hashOrVal.(map[string]interface{}); !ok {
			return false
		}
		if hashOrVal, ok = hash[k]; !ok {
			return false
		}
	}
	return true
}

// Type returns a string representation of the type of the key specified.
//
// Type will return the empty string if given an empty key or a key that does
// not exist. Keys are case sensitive.
func (md *MetaData) Type(key ...string) string {
	if t, ok := md.types[Key(key).String()]; ok {
		return t.String()
	}
	return ""
}

func (md *MetaData) TypeInfo(key ...string) tomlType {
	// TODO(v2): Type() would be a better name for this, but that's already
	//           used. We can change this to:
	//
	//   meta.TypeInfo()   → meta.Type()
	//   meta.IsDefined()  → meta.Type() == nil
	return md.types[Key(key).String()]
}

// Keys returns a slice of every key in the TOML data, including key groups.
//
// Each key is itself a slice, where the first element is the top of the
// hierarchy and the last is the most specific. The list will have the same
// order as the keys appeared in the TOML data.
//
// All keys returned are non-empty.
func (md *MetaData) Keys() []Key {
	return md.keys
}

// Undecoded returns all keys that have not been decoded in the order in which
// they appear in the original TOML document.
//
// This includes keys that haven't been decoded because of a Primitive value.
// Once the Primitive value is decoded, the keys will be considered decoded.
//
// Also note that decoding into an empty interface will result in no decoding,
// and so no keys will be considered decoded.
//
// In this sense, the Undecoded keys correspond to keys in the TOML document
// that do not have a concrete type in your representation.
func (md *MetaData) Undecoded() []Key {
	undecoded := make([]Key, 0, len(md.keys))
	for _, key := range md.keys {
		if !md.decoded[key.String()] {
			undecoded = append(undecoded, key)
		}
	}
	return undecoded
}

// Key represents any TOML key, including key groups. Use (MetaData).Keys to get
// values of this type.
type Key []string

func (k Key) String() string { return strings.Join(k, ".") }

func (k Key) maybeQuotedAll() string {
	var ss []string
	for i := range k {
		ss = append(ss, k.maybeQuoted(i))
	}
	return strings.Join(ss, ".")
}

func (k Key) maybeQuoted(i int) string {
	if k[i] == "" {
		return `""`
	}
	quote := false
	for _, c := range k[i] {
		if !isBareKeyChar(c) {
			quote = true
			break
		}
	}
	if quote {
		return `"` + dblQuotedReplacer.Replace(k[i]) + `"`
	}
	return k[i]
}

func (k Key) add(piece string) Key {
	newKey := make(Key, len(k)+1)
	copy(newKey, k)
	newKey[len(k)] = piece
	return newKey
}
