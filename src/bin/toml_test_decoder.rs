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
    if s.contains(':') && !s.contains('-') { return ("time-local", normalize_dt(s)); }
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
    // pad missing seconds: HH:MM -> HH:MM:00
    let s = pad_seconds(&s);
    // pad fractional seconds: .6 -> .600, .12 -> .120
    pad_frac(&s)
}

fn pad_seconds(s: &str) -> String {
    let time_start = if let Some(p) = s.find('T').or_else(|| s.find('t')) { p + 1 } else { 0 };
    let rest = &s[time_start..];
    let time_end = rest.find(|c: char| c == 'Z' || c == '+' || c == '-').unwrap_or(rest.len());
    let time_str = &rest[..time_end];
    let suffix = &rest[time_end..];
    let colon_count = time_str.matches(':').count();
    if colon_count == 1 && !time_str.contains('.') {
        return format!("{}{}:00{}", &s[..time_start + time_end], "", suffix);
    }
    s.to_string()
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
    else if *f == 0.0 { if f.is_sign_negative() { "-0".to_string() } else { "0".to_string() } }
    else {
        let abs = f.abs();
        // Use scientific notation for very large or very small numbers
        // toml-test expects: 5e+22, 1e+06, 6.626e-34, 3.0e14
        if abs >= 1e16 || (abs > 0.0 && abs < 1e-3) {
            // Scientific notation: match Go's strconv.FormatFloat(f, 'e', -1, 64)
            // Format as Xe+YY or Xe-YY with at least one decimal digit in mantissa
            let s = format!("{:e}", f);
            // Rust format: "5e22" -> need "5e+22", "6.626e-34" -> ok
            // Also "1e6" -> "1e+06" (pad exponent to 2 digits)
            normalize_scientific(&s)
        } else if f.fract() == 0.0 {
            // Integer-valued float: "300" not "300.0"
            format!("{}", *f as i64)
        } else {
            format!("{}", f)
        }
    }
}

/// Normalize Rust's scientific notation to match toml-test expected format.
/// Rust: "5e22" -> "5e+22", "1e6" -> "1e+06", "6.626e-34" -> "6.626e-34"
fn normalize_scientific(s: &str) -> String {
    if let Some(e_pos) = s.find('e') {
        let mantissa = &s[..e_pos];
        let exp = &s[e_pos+1..];
        // Ensure mantissa has at least one decimal digit
        let mantissa = if mantissa.contains('.') {
            mantissa.to_string()
        } else {
            format!("{}.0", mantissa)
        };
        // Parse and format exponent with sign and 2-digit padding
        let exp_val: i32 = exp.parse().unwrap_or(0);
        format!("{}e{:+03}", mantissa, exp_val)
    } else {
        s.to_string()
    }
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
    // Date: YYYY-MM-DD followed by T, space, or end
    if ch.len() >= 8 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit()
        && ch[2].is_ascii_digit() && ch[3].is_ascii_digit() && ch[4] == '-'
        && ch[5].is_ascii_digit() && ch[6].is_ascii_digit() && ch[7] == '-'
    {
        // Check the char after the date part is valid (T, space, t, end, or offset)
        if ch.len() == 10 { return true; } // pure date YYYY-MM-DD
        let next = ch[10];
        if next == 'T' || next == 't' || next == ' ' { return true; }
        if next == '+' || next == '-' { return true; } // offset
        return false; // 2020-01-01x is NOT a date
    }
    // Time: HH:MM
    if ch.len() >= 5 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit() && ch[2] == ':'
        && ch[3].is_ascii_digit() && ch[4].is_ascii_digit()
    { return true; }
    false
}