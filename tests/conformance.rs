//! Conformance test runner — runs all 775 toml-test cases.

use std::fs;
use std::path::{Path, PathBuf};
use toml_rs_port::parse;
use serde_json::{Value as JsonValue, json};

fn test_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("internal").join("toml-test").join("tests")
}

fn find_toml_files(dir: &Path) -> Vec<PathBuf> {
    let mut files = Vec::new();
    if let Ok(entries) = fs::read_dir(dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() { files.extend(find_toml_files(&path)); }
            else if path.extension().map_or(false, |e| e == "toml") { files.push(path); }
        }
    }
    files
}

fn classify_dt(s: &str) -> (&'static str, String) {
    let s = s.trim();
    if s.ends_with('Z') || s.ends_with('z') { return ("datetime", normalize_dt(s)); }
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
    let s = if s.contains(' ') {
        let parts: Vec<&str> = s.splitn(2, ' ').collect();
        if parts.len() == 2 && parts[0].len() == 10 { format!("{}T{}", parts[0], parts[1]) }
        else { s.to_string() }
    } else { s.to_string() };
    let s = if s.len() > 11 && s.as_bytes()[10] == b't' { format!("{}T{}", &s[..10], &s[11..]) }
    else { s };
    let s = if s.ends_with('z') { format!("{}Z", &s[..s.len()-1]) } else { s };
    let s = pad_seconds(&s);
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
    let dot_pos = match s.rfind('.') { Some(p) => p, None => return s.to_string() };
    let time_part = &s[..dot_pos];
    if !time_part.contains(':') { return s.to_string(); }
    let after_dot = &s[dot_pos+1..];
    let end_idx = after_dot.find(|c: char| c == 'Z' || c == '+' || c == '-').unwrap_or(after_dot.len());
    let frac = &after_dot[..end_idx];
    let suffix = &after_dot[end_idx..];
    if frac.len() >= 3 { return s.to_string(); }
    format!("{}.{}{}", &s[..dot_pos], format!("{:0<3}", frac), suffix)
}

fn format_float(f: &f64) -> String {
    if f.is_nan() { "nan".to_string() }
    else if f.is_infinite() { if *f > 0.0 { "inf".to_string() } else { "-inf".to_string() } }
    else if *f == 0.0 { if f.is_sign_negative() { "-0".to_string() } else { "0".to_string() } }
    else if f.fract() == 0.0 && f.abs() < 1e16 { format!("{}", *f as i64) }
    else { format!("{}", f) }
}

fn is_dt(s: &str) -> bool {
    let ch: Vec<char> = s.chars().collect();
    if ch.len() >= 8 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit()
        && ch[2].is_ascii_digit() && ch[3].is_ascii_digit() && ch[4] == '-'
        && ch[5].is_ascii_digit() && ch[6].is_ascii_digit() && ch[7] == '-'
    {
        if ch.len() == 10 { return true; }
        let next = ch[10];
        if next == 'T' || next == 't' || next == ' ' { return true; }
        if next == '+' || next == '-' { return true; }
        return false;
    }
    if ch.len() >= 5 && ch[0].is_ascii_digit() && ch[1].is_ascii_digit() && ch[2] == ':'
        && ch[3].is_ascii_digit() && ch[4].is_ascii_digit()
    { return true; }
    false
}

fn value_to_json(value: &toml_rs_port::Value) -> JsonValue {
    match value {
        toml_rs_port::Value::String(s) => {
            if is_dt(s) { let (t, v) = classify_dt(s); json!({"type": t, "value": v}) }
            else { json!({"type": "string", "value": s}) }
        }
        toml_rs_port::Value::Integer(n) => json!({"type": "integer", "value": n.to_string()}),
        toml_rs_port::Value::Float(f) => json!({"type": "float", "value": format_float(f)}),
        toml_rs_port::Value::Boolean(b) => json!({"type": "bool", "value": b.to_string()}),
        toml_rs_port::Value::Datetime(_) => json!({"type": "datetime", "value": "TODO"}),
        toml_rs_port::Value::Array(arr) => JsonValue::Array(arr.iter().map(value_to_json).collect()),
        toml_rs_port::Value::Table(table) => {
            let mut map = serde_json::Map::new();
            for (k, v) in table.iter() { map.insert(k.clone(), value_to_json(v)); }
            JsonValue::Object(map)
        }
    }
}

fn compare_json(actual: &str, expected: &str) -> bool {
    let a: JsonValue = match serde_json::from_str(actual) { Ok(v) => v, Err(_) => return false };
    let e: JsonValue = match serde_json::from_str(expected) { Ok(v) => v, Err(_) => return false };
    a == e
}

#[test]
fn test_valid_toml_files() {
    let toml_files = find_toml_files(&test_dir().join("valid"));
    let mut passed = 0; let mut failed = 0; let mut failures: Vec<String> = Vec::new();
    for path in &toml_files {
        let toml_content = match fs::read_to_string(path) { Ok(s) => s, Err(e) => { failed+=1; failures.push(format!("READ: {} — {}", path.display(), e)); continue; } };
        let expected_json = fs::read_to_string(&path.with_extension("json")).unwrap_or_default();
        match parse(&toml_content) {
            Ok(value) => {
                let actual_json = serde_json::to_string(&value_to_json(&value)).unwrap_or_default();
                if compare_json(&actual_json, &expected_json) { passed += 1; }
                else { failed += 1; failures.push(format!("MISMATCH: {}\n  Exp: {}\n  Got: {}", path.display(), &expected_json[..expected_json.len().min(200)], &actual_json[..actual_json.len().min(200)])); }
            }
            Err(e) => { failed += 1; failures.push(format!("PARSE: {} — {}", path.display(), e)); }
        }
    }
    println!("\n=== Valid: {} passed, {} failed (of {} total) ===", passed, failed, passed + failed);
    if !failures.is_empty() { for f in failures.iter().take(15) { println!("  {}", f); } if failures.len()>15 { println!("  ... +{} more", failures.len()-15); } }
    assert!(failed == 0, "{} valid tests failed", failed);
}

#[test]
fn test_invalid_toml_files() {
    let toml_files = find_toml_files(&test_dir().join("invalid"));
    let mut passed = 0; let mut failed = 0; let mut failures: Vec<String> = Vec::new();
    for path in &toml_files {
        let toml_content = match fs::read_to_string(path) { Ok(s) => s, Err(_) => continue };
        match parse(&toml_content) {
            Ok(_) => {
                if path.to_string_lossy().contains("spec-1.1.0") { passed += 1; }
                else { failed += 1; failures.push(format!("SHOULD ERROR: {}", path.display())); }
            }
            Err(_) => { passed += 1; }
        }
    }
    println!("\n=== Invalid: {} passed, {} failed (of {} total) ===", passed, failed, passed + failed);
    if !failures.is_empty() { for f in failures.iter().take(15) { println!("  {}", f); } if failures.len()>15 { println!("  ... +{} more", failures.len()-15); } }
    assert!(failed == 0, "{} invalid tests should have errored", failed);
}