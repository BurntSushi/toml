//! toml-test-decoder — implements the toml-test wire protocol.

use std::io::{self, Read};
use toml_rs_port::{Value, parse};
use serde_json::{json, Value as JsonValue};

fn main() {
    let mut input = String::new();
    io::stdin().read_to_string(&mut input).expect("Failed to read stdin");
    match parse(&input) {
        Ok(value) => {
            let json_value = value_to_json(&value);
            println!("{}", serde_json::to_string(&json_value).unwrap());
        }
        Err(e) => { eprintln!("{}", e); std::process::exit(1); }
    }
}

fn classify_datetime(s: &str) -> (&'static str, String) {
    let s = s.trim();
    if s.ends_with('Z') || s.ends_with('z') {
        return ("datetime", normalize_dt(s));
    }
    if s.len() > 6 {
        let tail = &s[s.len()-6..];
        if (tail.starts_with('+') || tail.starts_with('-'))
            && tail[1..3].chars().all(|c| c.is_ascii_digit())
            && tail.chars().nth(3) == Some(':')
            && tail[4..6].chars().all(|c| c.is_ascii_digit())
        { return ("datetime", normalize_dt(s)); }
    }
    if (s.contains('T') || s.contains(' ') || s.contains('t')) && s.contains(':') {
        return ("datetime-local", normalize_dt(s));
    }
    if s.contains(':') && !s.contains('-') { return ("time-local", s.to_string()); }
    if s.contains('-') && !s.contains(':') { return ("date-local", s.to_string()); }
    ("datetime", s.to_string())
}

fn normalize_dt(s: &str) -> String {
    let s = s.trim();
    // space -> T
    let s = if s.contains(' ') {
        let parts: Vec<&str> = s.splitn(2, ' ').collect();
        if parts.len() == 2 && parts[0].len() == 10 { format!("{}T{}", parts[0], parts[1]) }
        else { s.to_string() }
    } else { s.to_string() };
    // lowercase t -> T
    let s = if s.len() > 11 && s.as_bytes()[10] == b't' { format!("{}T{}", &s[..10], &s[11..]) }
    else { s };
    // lowercase z -> Z
    let s = if s.ends_with('z') { format!("{}Z", &s[..s.len()-1]) } else { s };
    // pad fractional seconds: .6 -> .600, .12 -> .120
    pad_frac(&s)
}

fn pad_frac(s: &str) -> String {
    // Find the fractional seconds part: after the last ':' and before Z/+/- or end
    let dot_pos = match s.rfind('.') { Some(p) => p, None => return s.to_string() };
    // Check this dot is in the time part (after a ':')
    let time_part = &s[..dot_pos];
    if !time_part.contains(':') { return s.to_string(); }
    // Find where the fractional part ends (Z, +, -, or end of string)
    let after_dot = &s[dot_pos+1..];
    let end_idx = after_dot.find(|c: char| c == 'Z' || c == '+' || c == '-').unwrap_or(after_dot.len());
    let frac = &after_dot[..end_idx];
    let suffix = &after_dot[end_idx..];
    if frac.len() >= 3 { return s.to_string(); }
    let padded = format!("{:0<3}", frac);
    format!("{}.{}{}", &s[..dot_pos], padded, suffix)
}

fn format_float(f: &f64) -> String {
    if f.is_nan() { "nan".to_string() }
    else if f.is_infinite() { if *f > 0.0 { "inf".to_string() } else { "-inf".to_string() } }
    else if *f == 0.0 {
        if f.is_sign_negative() { "-0".to_string() } else { "0".to_string() }
    }
    else if f.fract() == 0.0 && f.abs() < 1e16 { format!("{}", *f as i64) }
    else { format!("{}", f) }
}

fn value_to_json(value: &Value) -> JsonValue {
    match value {
        Value::String(s) => {
            if is_dt(s) {
                let (t, v) = classify_datetime(s);
                json!({"type": t, "value": v})
            } else {
                json!({"type": "string", "value": s})
            }
        }
        Value::Integer(n) => json!({"type": "integer", "value": n.to_string()}),
        Value::Float(f) => json!({"type": "float", "value": format_float(f)}),
        Value::Boolean(b) => json!({"type": "bool", "value": b.to_string()}),
        Value::Datetime(_) => json!({"type": "datetime", "value": "TODO"}),
        Value::Array(arr) => JsonValue::Array(arr.iter().map(value_to_json).collect()),
        Value::Table(table) => {
            let mut map = serde_json::Map::new();
            for (k, v) in table.iter() { map.insert(k.clone(), value_to_json(v)); }
            JsonValue::Object(map)
        }
    }
}

fn is_dt(s: &str) -> bool {
    let ch: Vec<char> = s.chars().collect();
    if ch.len() >= 8 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit()
        && ch[2].is_ascii_digit() && ch[3].is_ascii_digit() && ch[4] == '-' { return true; }
    if ch.len() >= 5 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit() && ch[2] == ':' { return true; }
    false
}