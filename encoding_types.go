// +build go1.2

package toml

// In order to support Go 1.1, we define our own TextMarshaler and
// TextUnmarshaler types. For Go 1.2+, we just alias them with the
// standard library interfaces.

import (
	"encoding"
)

type TextMarshaler encoding.TextMarshaler
type TextUnmarshaler encoding.TextUnmarshaler
