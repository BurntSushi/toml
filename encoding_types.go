package toml

// In order to support Go 1.1, we define our own TextMarshaler and
// TextUnmarshaler types. For Go 1.2+, we just alias them with the
// standard library interfaces.
//
// Note this is no longer needed, but since these types are exported we can't
// really remove them without breaking compatibility. I don't think many people
// referenced these interfaces directly, but it's hard to be sure and it does
// little harm just keeping them here.

import "encoding"

// TextMarshaler is a synonym for encoding.TextMarshaler. It is defined here
// to support Go 1.1.
//
// This is deprecated and should no longer be used. Use the identical
// encoding.TextMarshaler instead.
type TextMarshaler encoding.TextMarshaler

// TextUnmarshaler is a synonym for encoding.TextUnmarshaler. It is defined
// here so that Go 1.1 can be supported.
//
// This is deprecated and should no longer be used. Use the identical
// encoding.TextUnmarshaler instead.
type TextUnmarshaler encoding.TextUnmarshaler
