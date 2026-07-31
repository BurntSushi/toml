//! Decoder — ported from decode.go (648 LOC)
//! Struct deserialization layer (not needed for toml-test conformance — only Value-level)

use crate::Value;
use crate::error::ParseError;

/// Decode a TOML string into a Value tree.
/// This is the primary entry point for the toml-test protocol.
pub fn decode(input: &str) -> Result<Value, ParseError> {
    crate::parse(input)
}
