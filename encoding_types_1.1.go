// +build !go1.2

package toml

// These interfaces were introduced in Go 1.2, so we add them manually when
// compiling for Go 1.1.

type TextMarshaler interface {
	MarshalText() (text []byte, err error)
}

type TextUnmarshaler interface {
	UnmarshalText(text []byte) error
}
