//! TOML → Rust port of BurntSushi/toml
//!
//! A TOML 1.0.0 parser and encoder, ported from Go to idiomatic Rust.
//!
//! The core library exposes `parse()` which returns a `Value` tree.
//! The `toml-test-decoder` binary implements the toml-test wire protocol
//! (TOML via stdin → JSON via stdout) for conformance testing.

pub mod lex;
pub mod parse;
pub mod decode;
pub mod encode;
pub mod error;
pub mod meta;
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