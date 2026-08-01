//! toml-test-decoder — implements the toml-test wire protocol.
//!
//! Reads TOML on stdin, writes tagged JSON on stdout, exits non-zero with a
//! message on stderr for invalid input.
//!
//! All type tagging lives in the library (`Value::type_tag`), not here — a
//! decoder that re-derives types by inspecting strings will happily report a
//! quoted `"1979-05-27"` as a date.

use serde_json::{json, Value as JsonValue};
use std::io::{self, Read};
use toml_rs_port::{parse, Value};

fn main() {
    let mut raw = Vec::new();
    if let Err(e) = io::stdin().read_to_end(&mut raw) {
        eprintln!("toml: error: cannot read stdin: {}", e);
        std::process::exit(1);
    }
    // TOML documents must be valid UTF-8; anything else is a decode error,
    // not a crash.
    let input = match String::from_utf8(raw) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("toml: error: input is not valid UTF-8: {}", e);
            std::process::exit(1);
        }
    };
    match parse(&input) {
        Ok(value) => println!("{}", serde_json::to_string(&value_to_json(&value)).unwrap()),
        Err(e) => {
            eprintln!("{}", e);
            std::process::exit(1);
        }
    }
}

fn value_to_json(value: &Value) -> JsonValue {
    match value {
        Value::Array(arr) => JsonValue::Array(arr.iter().map(value_to_json).collect()),
        Value::Table(table) => {
            let mut map = serde_json::Map::new();
            for (k, v) in table.iter() {
                map.insert(k.clone(), value_to_json(v));
            }
            JsonValue::Object(map)
        }
        scalar => json!({
            "type": scalar.type_tag().expect("scalars always carry a type tag"),
            "value": scalar.value_string().expect("scalars always render a value"),
        }),
    }
}
