//! toml-test-decoder — implements the toml-test wire protocol.
//!
//! Accepts TOML on stdin, outputs JSON on stdout.
//! For invalid TOML, outputs an error to stderr and exits non-zero.

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

/// Classify a datetime string into the toml-test type tags.
/// Returns ("type", value_string)
fn classify_datetime(s: &str) -> (&'static str, String) {
    let s = s.trim();
    // Has timezone (Z or +HH:MM or -HH:MM)
    if s.ends_with('Z') || s.ends_with('z') {
        return ("datetime", normalize_datetime(s));
    }
    // Check for +HH:MM or -HH:MM at the end (timezone offset)
    if s.len() > 6 {
        let tail = &s[s.len()-6..];
        if (tail.starts_with('+') || tail.starts_with('-'))
            && tail[1..3].chars().all(|c| c.is_ascii_digit())
            && tail.chars().nth(3) == Some(':')
            && tail[4..6].chars().all(|c| c.is_ascii_digit())
        {
            return ("datetime", normalize_datetime(s));
        }
    }
    // Has both date and time (T or space separator)
    if (s.contains('T') || s.contains(' ')) && s.contains(':') {
        return ("datetime-local", normalize_datetime(s));
    }
    // Has time only (contains : but no -)
    if s.contains(':') && !s.contains('-') {
        return ("time-local", s.to_string());
    }
    // Has date only (contains - but no :)
    if s.contains('-') && !s.contains(':') {
        return ("date-local", s.to_string());
    }
    // Fallback
    ("datetime", s.to_string())
}

/// Normalize datetime: convert space separator to T, lowercase z to Z
fn normalize_datetime(s: &str) -> String {
    let s = s.trim();
    // Replace space separator with T
    let s = if s.contains(' ') {
        // Only replace the space between date and time
        let parts: Vec<&str> = s.splitn(2, ' ').collect();
        if parts.len() == 2 && parts[0].len() == 10 {
            format!("{}T{}", parts[0], parts[1])
        } else {
            s.to_string()
        }
    } else {
        s.to_string()
    };
    // Uppercase Z
    if s.ends_with('z') {
        format!("{}Z", &s[..s.len()-1])
    } else {
        s
    }
}

/// Format a float for toml-test output.
fn format_float(f: &f64) -> String {
    if f.is_nan() {
        "nan".to_string()
    } else if f.is_infinite() {
        if *f > 0.0 { "inf".to_string() } else { "-inf".to_string() }
    } else if *f == 0.0 {
        // Handle +0.0 and -0.0
        if f.is_sign_negative() { "-0".to_string() } else { "0".to_string() }
    } else if f.fract() == 0.0 && f.abs() < 1e16 {
        // Integer-valued float: "300" not "300.0"
        format!("{}", *f as i64)
    } else {
        // Regular float
        let s = format!("{}", f);
        s
    }
}

/// Convert a TOML Value tree to the toml-test JSON format.
fn value_to_json(value: &Value) -> JsonValue {
    match value {
        Value::String(s) => {
            // Check if this is actually a datetime string
            if is_datetime_string(s) {
                let (dt_type, dt_val) = classify_datetime(s);
                json!({"type": dt_type, "value": dt_val})
            } else {
                json!({"type": "string", "value": s})
            }
        }
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

/// Check if a string looks like a datetime
fn is_datetime_string(s: &str) -> bool {
    let chars: Vec<char> = s.chars().collect();
    // Date: YYYY-MM-DD...
    if chars.len() >= 8
        && chars[0].is_ascii_digit()
        && chars[1].is_ascii_digit()
        && chars[2].is_ascii_digit()
        && chars[3].is_ascii_digit()
        && chars[4] == '-'
    {
        return true;
    }
    // Time: HH:MM...
    if chars.len() >= 5
        && chars[0].is_ascii_digit()
        && chars[1].is_ascii_digit()
        && chars[2] == ':'
    {
        return true;
    }
    false
}