//! Encoder — ported from encode.go (784 LOC)
//! Encodes a Value tree back to a TOML string.

use crate::Value;
use crate::error::EncodeError;


pub fn encode(value: &Value) -> Result<String, EncodeError> {
    let mut out = String::new();
    encode_value(value, &mut out, 0)?;
    Ok(out)
}

fn encode_value(value: &Value, out: &mut String, indent: usize) -> Result<(), EncodeError> {
    match value {
        Value::String(s) => {
            out.push('"');
            for c in s.chars() {
                match c {
                    '"' => out.push_str("\\\""),
                    '\\' => out.push_str("\\\\"),
                    '\n' => out.push_str("\\n"),
                    '\t' => out.push_str("\\t"),
                    '\r' => out.push_str("\\r"),
                    c if c.is_control() => {
                        out.push_str(&format!("\\u{:04X}", c as u32));
                    }
                    c => out.push(c),
                }
            }
            out.push('"');
        }
        Value::Integer(n) => out.push_str(&n.to_string()),
        Value::Float(f) => out.push_str(&f.to_string()),
        Value::Boolean(b) => out.push_str(if *b { "true" } else { "false" }),
        Value::Array(arr) => {
            out.push('[');
            for (i, v) in arr.iter().enumerate() {
                if i > 0 { out.push_str(", "); }
                encode_value(v, out, indent)?;
            }
            out.push(']');
        }
        Value::Datetime(_) => {
            out.push_str("TODO: datetime encoding");
        }
        Value::Table(table) => {
            for (key, val) in table.iter() {
                out.push_str(key);
                out.push_str(" = ");
                encode_value(val, out, indent)?;
                out.push('\n');
            }
        }
    }
    Ok(())
}
