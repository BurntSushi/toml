//! Encoder — ported from encode.go (784 LOC)
//! Encodes a Value tree back to a TOML string.

use crate::error::EncodeError;
use crate::number::format_float;
use crate::Value;
use std::collections::BTreeMap;

/// Encode a value tree as a TOML document.
///
/// Scalars and arrays are written first, then sub-tables as `[header]`
/// sections and arrays of tables as `[[header]]` — the layout the Go original
/// produces, and the only one that round-trips for nested tables.
pub fn encode(value: &Value) -> Result<String, EncodeError> {
    let table = match value {
        Value::Table(t) => t,
        _ => return Err(EncodeError::InvalidValue("top-level value must be a table".into())),
    };
    let mut out = String::new();
    encode_table(table, &mut Vec::new(), &mut out)?;
    Ok(out)
}

fn encode_table(
    table: &BTreeMap<String, Value>,
    path: &mut Vec<String>,
    out: &mut String,
) -> Result<(), EncodeError> {
    // Pass 1: everything that fits on a `key = value` line.
    for (key, val) in table {
        if is_section(val) {
            continue;
        }
        out.push_str(&encode_key(key));
        out.push_str(" = ");
        encode_inline(val, out)?;
        out.push('\n');
    }

    // Pass 2: sub-tables and arrays of tables.
    for (key, val) in table {
        match val {
            Value::Table(sub) => {
                path.push(key.clone());
                if !out.is_empty() && !out.ends_with("\n\n") {
                    out.push('\n');
                }
                out.push_str(&format!("[{}]\n", encode_path(path)));
                encode_table(sub, path, out)?;
                path.pop();
            }
            Value::Array(items) if is_table_array(items) => {
                path.push(key.clone());
                for item in items {
                    let Value::Table(sub) = item else { continue };
                    if !out.is_empty() && !out.ends_with("\n\n") {
                        out.push('\n');
                    }
                    out.push_str(&format!("[[{}]]\n", encode_path(path)));
                    encode_table(sub, path, out)?;
                }
                path.pop();
            }
            _ => {}
        }
    }
    Ok(())
}

/// Whether this value must be written as a `[header]` section rather than
/// inline. An empty array is ambiguous, so it stays inline.
fn is_section(v: &Value) -> bool {
    match v {
        Value::Table(_) => true,
        Value::Array(items) => is_table_array(items),
        _ => false,
    }
}

fn is_table_array(items: &[Value]) -> bool {
    !items.is_empty() && items.iter().all(|v| matches!(v, Value::Table(_)))
}

fn encode_path(path: &[String]) -> String {
    path.iter().map(|k| encode_key(k)).collect::<Vec<_>>().join(".")
}

/// Bare where possible, quoted where not.
fn encode_key(key: &str) -> String {
    let bare = !key.is_empty()
        && key.chars().all(|c| c.is_ascii_alphanumeric() || c == '_' || c == '-');
    if bare {
        key.to_string()
    } else {
        encode_string(key)
    }
}

fn encode_inline(value: &Value, out: &mut String) -> Result<(), EncodeError> {
    match value {
        Value::String(s) => out.push_str(&encode_string(s)),
        Value::Integer(n) => out.push_str(&n.to_string()),
        Value::Float(f, _) => out.push_str(&encode_float(*f)),
        Value::Boolean(b) => out.push_str(if *b { "true" } else { "false" }),
        Value::Datetime(dt) => out.push_str(&dt.to_string()),
        Value::Array(arr) => {
            out.push('[');
            for (i, v) in arr.iter().enumerate() {
                if i > 0 {
                    out.push_str(", ");
                }
                encode_inline(v, out)?;
            }
            out.push(']');
        }
        Value::Table(t) => {
            // Reached only for a table nested inside an array.
            out.push_str("{ ");
            for (i, (k, v)) in t.iter().enumerate() {
                if i > 0 {
                    out.push_str(", ");
                }
                out.push_str(&encode_key(k));
                out.push_str(" = ");
                encode_inline(v, out)?;
            }
            out.push_str(" }");
        }
    }
    Ok(())
}

/// TOML floats need a fraction or an exponent, so a value like `1` that came
/// in as a float has to be written `1.0` to survive a round trip.
fn encode_float(f: f64) -> String {
    let s = format_float(f);
    if s.contains(['.', 'e', 'n', 'i']) {
        s
    } else {
        format!("{}.0", s)
    }
}

fn encode_string(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\t' => out.push_str("\\t"),
            '\r' => out.push_str("\\r"),
            '\u{0008}' => out.push_str("\\b"),
            '\u{000C}' => out.push_str("\\f"),
            c if (c as u32) < 0x20 || (c as u32) == 0x7F => {
                out.push_str(&format!("\\u{:04X}", c as u32));
            }
            c => out.push(c),
        }
    }
    out.push('"');
    out
}
