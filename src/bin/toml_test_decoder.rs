//! toml-test-decoder — implements the toml-test wire protocol.
//!
//! Accepts TOML on stdin, outputs JSON on stdout.
//! For invalid TOML, outputs an error to stderr and exits non-zero.
//!
//! This is the binary that the 775 toml-test conformance cases run against.

use std::io::{self, Read};
use toml_rs_port::{Value, parse};
use serde_json::{json, Value as JsonValue};

fn main() {
    let mut input = String::new();
    io::stdin().read_to_string(&mut input).expect("Failed to read stdin");

    match parse(&input) {
        Ok(value) => {
            let json_value = value_to_json(&value);
            let json_str = serde_json::to_string(&json_value).unwrap();
            println!("{}", json_str);
        }
        Err(e) => {
            eprintln!("{}", e);
            std::process::exit(1);
        }
    }
}

/// Convert a TOML Value tree to the toml-test JSON format.
///
/// The toml-test protocol wraps each value in a type-tagged JSON object:
///   - Strings: {"type": "string", "value": "..."}
///   - Integers: {"type": "integer", "value": "42"}
///   - Floats: {"type": "float", "value": "3.14"}
///   - Booleans: {"type": "bool", "value": "true"}
///   - Datetimes: {"type": "datetime", "value": "2023-01-01T00:00:00Z"}
///   - Arrays: [wrapped_value, ...]
///   - Tables: {"key": wrapped_value, ...}
fn value_to_json(value: &Value) -> JsonValue {
    match value {
        Value::String(s) => json!({"type": "string", "value": s}),
        Value::Integer(n) => json!({"type": "integer", "value": n.to_string()}),
        Value::Float(f) => json!({"type": "float", "value": format_float(f)}),
        Value::Boolean(b) => json!({"type": "bool", "value": b.to_string()}),
        Value::Datetime(_) => json!({"type": "datetime", "value": "TODO"}),
        Value::Array(arr) => {
            JsonValue::Array(arr.iter().map(value_to_json).collect())
        }
        Value::Table(table) => {
            let mut map = serde_json::Map::new();
            for (k, v) in table.iter() {
                map.insert(k.clone(), value_to_json(v));
            }
            JsonValue::Object(map)
        }
    }
}

/// Format a float for toml-test output.
fn format_float(f: &f64) -> String {
    if f.is_nan() {
        "nan".to_string()
    } else if f.is_infinite() {
        if *f > 0.0 { "inf".to_string() } else { "-inf".to_string() }
    } else if f.fract() == 0.0 && f.abs() < 1e16 {
        format!("{:.1}", f) // e.g., 42.0
    } else {
        f.to_string()
    }
}
