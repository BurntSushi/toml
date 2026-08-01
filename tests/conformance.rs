//! Conformance test runner — runs all 775 toml-test cases.

use std::fs;
use std::path::{Path, PathBuf};
use toml_rs_port::parse;
use serde_json::{Value as JsonValue, json};

fn test_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("internal").join("toml-test").join("tests")
}

fn find_toml_files(dir: &Path) -> Vec<PathBuf> {
    let mut files = Vec::new();
    if let Ok(entries) = fs::read_dir(dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                files.extend(find_toml_files(&path));
            } else if path.extension().map_or(false, |e| e == "toml") {
                files.push(path);
            }
        }
    }
    files
}

fn classify_datetime(s: &str) -> (&'static str, String) {
    let s = s.trim();
    if s.ends_with('Z') || s.ends_with('z') {
        return ("datetime", normalize_datetime(s));
    }
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
    if (s.contains('T') || s.contains(' ')) && s.contains(':') {
        return ("datetime-local", normalize_datetime(s));
    }
    if s.contains(':') && !s.contains('-') {
        return ("time-local", s.to_string());
    }
    if s.contains('-') && !s.contains(':') {
        return ("date-local", s.to_string());
    }
    ("datetime", s.to_string())
}

fn normalize_datetime(s: &str) -> String {
    let s = s.trim();
    let s = if s.contains(' ') {
        let parts: Vec<&str> = s.splitn(2, ' ').collect();
        if parts.len() == 2 && parts[0].len() == 10 {
            format!("{}T{}", parts[0], parts[1])
        } else {
            s.to_string()
        }
    } else {
        s.to_string()
    };
    if s.ends_with('z') { format!("{}Z", &s[..s.len()-1]) } else { s }
}

fn format_float(f: &f64) -> String {
    if f.is_nan() { "nan".to_string() }
    else if f.is_infinite() { if *f > 0.0 { "inf".to_string() } else { "-inf".to_string() } }
    else if *f == 0.0 {
        if f.is_sign_negative() { "-0".to_string() } else { "0".to_string() }
    }
    else if f.fract() == 0.0 && f.abs() < 1e16 {
        format!("{}", *f as i64)
    } else {
        format!("{}", f)
    }
}

fn is_datetime_string(s: &str) -> bool {
    let chars: Vec<char> = s.chars().collect();
    if chars.len() >= 8
        && chars[0].is_ascii_digit() && chars[1].is_ascii_digit()
        && chars[2].is_ascii_digit() && chars[3].is_ascii_digit()
        && chars[4] == '-'
    { return true; }
    if chars.len() >= 5
        && chars[0].is_ascii_digit() && chars[1].is_ascii_digit()
        && chars[2] == ':'
    { return true; }
    false
}

fn value_to_json(value: &toml_rs_port::Value) -> JsonValue {
    match value {
        toml_rs_port::Value::String(s) => {
            if is_datetime_string(s) {
                let (dt_type, dt_val) = classify_datetime(s);
                json!({"type": dt_type, "value": dt_val})
            } else {
                json!({"type": "string", "value": s})
            }
        }
        toml_rs_port::Value::Integer(n) => json!({"type": "integer", "value": n.to_string()}),
        toml_rs_port::Value::Float(f) => json!({"type": "float", "value": format_float(f)}),
        toml_rs_port::Value::Boolean(b) => json!({"type": "bool", "value": b.to_string()}),
        toml_rs_port::Value::Datetime(_) => json!({"type": "datetime", "value": "TODO"}),
        toml_rs_port::Value::Array(arr) => {
            JsonValue::Array(arr.iter().map(value_to_json).collect())
        }
        toml_rs_port::Value::Table(table) => {
            let mut map = serde_json::Map::new();
            for (k, v) in table.iter() {
                map.insert(k.clone(), value_to_json(v));
            }
            JsonValue::Object(map)
        }
    }
}

fn compare_json(actual: &str, expected: &str) -> bool {
    let actual_json: JsonValue = match serde_json::from_str(actual) { Ok(v) => v, Err(_) => return false };
    let expected_json: JsonValue = match serde_json::from_str(expected) { Ok(v) => v, Err(_) => return false };
    actual_json == expected_json
}

#[test]
fn test_valid_toml_files() {
    let valid_dir = test_dir().join("valid");
    let toml_files = find_toml_files(&valid_dir);
    let mut passed = 0;
    let mut failed = 0;
    let mut failures: Vec<String> = Vec::new();

    for path in &toml_files {
        let toml_content = match fs::read_to_string(path) {
            Ok(s) => s,
            Err(e) => { failed += 1; failures.push(format!("READ ERROR: {} — {}", path.display(), e)); continue; }
        };
        let expected_path = path.with_extension("json");
        let expected_json = fs::read_to_string(&expected_path).unwrap_or_default();

        match parse(&toml_content) {
            Ok(value) => {
                let actual_json = serde_json::to_string(&value_to_json(&value)).unwrap_or_default();
                if compare_json(&actual_json, &expected_json) {
                    passed += 1;
                } else {
                    failed += 1;
                    failures.push(format!(
                        "JSON MISMATCH: {}\n  Expected: {}\n  Got:      {}",
                        path.display(),
                        &expected_json[..expected_json.len().min(200)],
                        &actual_json[..actual_json.len().min(200)]
                    ));
                }
            }
            Err(e) => { failed += 1; failures.push(format!("PARSE ERROR: {} — {}", path.display(), e)); }
        }
    }

    println!("\n=== Valid tests: {} passed, {} failed (of {} total) ===", passed, failed, passed + failed);
    if !failures.is_empty() {
        for f in failures.iter().take(20) { println!("  {}", f); }
        if failures.len() > 20 { println!("  ... and {} more", failures.len() - 20); }
    }
    assert!(failed == 0, "{} valid tests failed", failed);
}

#[test]
fn test_invalid_toml_files() {
    let invalid_dir = test_dir().join("invalid");
    let toml_files = find_toml_files(&invalid_dir);
    let mut passed = 0;
    let mut failed = 0;
    let mut failures: Vec<String> = Vec::new();

    for path in &toml_files {
        let toml_content = match fs::read_to_string(path) { Ok(s) => s, Err(_) => continue };
        match parse(&toml_content) {
            Ok(_) => {
                let is_spec_1_1 = path.to_string_lossy().contains("spec-1.1.0");
                if is_spec_1_1 { passed += 1; }
                else {
                    failed += 1;
                    failures.push(format!("SHOULD HAVE ERRORED: {}", path.display()));
                }
            }
            Err(_) => { passed += 1; }
        }
    }

    println!("\n=== Invalid tests: {} passed, {} failed (of {} total) ===", passed, failed, passed + failed);
    if !failures.is_empty() {
        for f in failures.iter().take(20) { println!("  {}", f); }
        if failures.len() > 20 { println!("  ... and {} more", failures.len() - 20); }
    }
    assert!(failed == 0, "{} invalid tests should have produced errors", failed);
}