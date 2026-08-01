//! toml-test-encoder — implements the toml-test encoder protocol.
//! Accepts tagged JSON on stdin, outputs TOML on stdout.

use serde_json::Value as JsonValue;
use std::collections::BTreeMap;
use std::io::{self, Read};
use toml_rs_port::datetime::parse_datetime;
use toml_rs_port::{encode, Value};

fn main() {
    let mut input = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut input) {
        eprintln!("toml: error: cannot read stdin: {}", e);
        std::process::exit(1);
    }
    let json: JsonValue = match serde_json::from_str(&input) {
        Ok(j) => j,
        Err(e) => {
            eprintln!("toml: error: input is not valid JSON: {}", e);
            std::process::exit(1);
        }
    };
    match json_to_value(&json).and_then(|v| encode(&v).map_err(|e| e.to_string())) {
        Ok(toml) => print!("{}", toml),
        Err(e) => {
            eprintln!("toml: error: {}", e);
            std::process::exit(1);
        }
    }
}

/// Rebuild a `Value` tree from toml-test's tagged JSON.
fn json_to_value(json: &JsonValue) -> Result<Value, String> {
    match json {
        JsonValue::Array(items) => items
            .iter()
            .map(json_to_value)
            .collect::<Result<Vec<_>, _>>()
            .map(Value::Array),
        JsonValue::Object(map) => {
            if let (Some(JsonValue::String(t)), Some(JsonValue::String(v))) =
                (map.get("type"), map.get("value"))
            {
                if map.len() == 2 {
                    return scalar(t, v);
                }
            }
            let mut table = BTreeMap::new();
            for (k, v) in map {
                table.insert(k.clone(), json_to_value(v)?);
            }
            Ok(Value::Table(table))
        }
        other => Err(format!("unexpected JSON value: {}", other)),
    }
}

fn scalar(tag: &str, raw: &str) -> Result<Value, String> {
    match tag {
        "string" => Ok(Value::String(raw.to_string())),
        "integer" => raw
            .parse::<i64>()
            .map(Value::Integer)
            .map_err(|e| format!("bad integer {:?}: {}", raw, e)),
        "float" => parse_float(raw).map(|f| Value::Float(f, raw.to_string())),
        "bool" => match raw.to_ascii_lowercase().as_str() {
            "true" => Ok(Value::Boolean(true)),
            "false" => Ok(Value::Boolean(false)),
            _ => Err(format!("bad bool {:?}", raw)),
        },
        "datetime" | "datetime-local" | "date-local" | "time-local" => parse_datetime(raw, 0, 0)
            .map(Value::Datetime)
            .map_err(|e| e.to_string()),
        other => Err(format!("unknown type tag {:?}", other)),
    }
}

fn parse_float(raw: &str) -> Result<f64, String> {
    match raw.to_ascii_lowercase().as_str() {
        "inf" | "+inf" => Ok(f64::INFINITY),
        "-inf" => Ok(f64::NEG_INFINITY),
        "nan" | "+nan" => Ok(f64::NAN),
        "-nan" => Ok(-f64::NAN),
        s => s.parse().map_err(|e| format!("bad float {:?}: {}", raw, e)),
    }
}
