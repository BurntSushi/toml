//! TOML → Rust port of BurntSushi/toml
//!
//! A TOML 1.0.0 parser and encoder, ported from Go to idiomatic Rust.
//!
//! The core library exposes `parse()` which returns a `Value` tree.
//! The `toml-test-decoder` binary implements the toml-test wire protocol
//! (TOML via stdin → JSON via stdout) for conformance testing.

pub mod datetime;
pub mod decode;
pub mod encode;
pub mod error;
pub mod lex;
pub mod meta;
pub mod number;
pub mod parse;
pub mod types;

/// A TOML value — the root type returned by parsing.
///
/// This is the Rust equivalent of Go's `interface{}` used in the original
/// to represent any TOML value. Using a sum type instead of dynamic typing
/// eliminates runtime type-assertion panics.
#[derive(Debug, Clone, PartialEq)]
pub enum Value {
    String(String),
    Integer(i64),
    Float(f64, String),
    Boolean(bool),
    Datetime(Datetime),
    Array(Vec<Value>),
    Table(BTreeMap<String, Value>),
}

/// A TOML datetime value.
///
/// Unlike the Go original which uses `time.Time` for all datetime variants,
/// Rust uses distinct variants to enforce the type distinction at compile time.
#[derive(Debug, Clone, PartialEq)]
pub enum Datetime {
    /// An offset date-time: `2023-01-01T12:00:00Z`
    Offset {
        date: Date,
        time: Time,
        offset: TimeOffset,
    },
    /// A local date-time: `2023-01-01T12:00:00`
    Local {
        date: Date,
        time: Time,
    },
    /// A local date: `2023-01-01`
    DateOnly(Date),
    /// A local time: `12:00:00`
    TimeOnly(Time),
}

#[derive(Debug, Clone, PartialEq)]
pub struct Date {
    pub year: i32,
    pub month: u8,
    pub day: u8,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Time {
    pub hour: u8,
    pub minute: u8,
    pub second: u8,
    pub nanosecond: u32,
}

#[derive(Debug, Clone, PartialEq)]
pub enum TimeOffset {
    Z,
    Offset(i16), // minutes from UTC
}

use std::collections::BTreeMap;

impl Value {
    /// The toml-test type tag for this value, or `None` for the composite
    /// kinds (array and table) which carry no tag of their own.
    ///
    /// Type tagging lives here rather than in the test harness so that a
    /// quoted string that merely *looks* like a datetime stays a string.
    pub fn type_tag(&self) -> Option<&'static str> {
        match self {
            Value::String(_) => Some("string"),
            Value::Integer(_) => Some("integer"),
            Value::Float(..) => Some("float"),
            Value::Boolean(_) => Some("bool"),
            Value::Datetime(dt) => Some(dt.type_tag()),
            Value::Array(_) | Value::Table(_) => None,
        }
    }

    /// The toml-test `value` field: every scalar is rendered as a string.
    pub fn value_string(&self) -> Option<String> {
        match self {
            Value::String(s) => Some(s.clone()),
            Value::Integer(n) => Some(n.to_string()),
            Value::Float(f, _) => Some(number::format_float(*f)),
            Value::Boolean(b) => Some(b.to_string()),
            Value::Datetime(dt) => Some(dt.to_string()),
            Value::Array(_) | Value::Table(_) => None,
        }
    }
}

/// Parse a TOML string into a `Value` tree.
///
/// This is the primary entry point, equivalent to `toml.Decode()` in Go.
pub fn parse(input: &str) -> Result<Value, error::ParseError> {
    let tokens = lex::lex(input)?;
    parse::parse(tokens)
}

/// Encode a `Value` tree back to a TOML string.
///
/// Equivalent to `toml.Encode()` in Go.
pub fn encode(value: &Value) -> Result<String, error::EncodeError> {
    encode::encode(value)
}
#[cfg(test)]
mod tests {
    use super::*;

    /// A quoted string that happens to look like a datetime is a string.
    ///
    /// Regression test: an earlier revision stored datetimes as `Value::String`
    /// and let the test harness recover the type by inspecting the text, so
    /// `a = "1979-05-27"` was reported as a `date-local`. The conformance
    /// corpus does not cover this, so it passed.
    #[test]
    fn quoted_strings_that_look_like_datetimes_stay_strings() {
        let doc = r#"
            a = "1979-05-27T07:32:00Z"
            b = "12:34:56"
            c = "1979-05-27"
            d = 1979-05-27
        "#;
        let Value::Table(t) = parse(doc).expect("should parse") else { panic!("not a table") };
        assert_eq!(t["a"].type_tag(), Some("string"));
        assert_eq!(t["b"].type_tag(), Some("string"));
        assert_eq!(t["c"].type_tag(), Some("string"));
        // The bare literal really is a date.
        assert_eq!(t["d"].type_tag(), Some("date-local"));
        assert_eq!(t["c"].value_string().unwrap(), "1979-05-27");
    }

    #[test]
    fn round_trips_through_the_encoder() {
        let doc = "\
title = \"x\"\n\
[a]\n\
b = 1\n\
[[a.c]]\n\
d = 2.5\n\
[[a.c]]\n\
d = 1979-05-27T07:32:00Z\n";
        let parsed = parse(doc).expect("should parse");
        let encoded = encode(&parsed).expect("should encode");
        assert_eq!(parse(&encoded).expect("should re-parse"), parsed);
    }
}
