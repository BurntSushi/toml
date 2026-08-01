//! Conformance test runner — runs all 775 toml-test cases.
//!
//! Comparison follows the reference runner in `internal/toml-test/json.go`
//! rather than comparing JSON as text:
//!
//!   * floats are compared numerically (`cmpFloats`), so `3e+14` and `3.0e14`
//!     are the same value;
//!   * datetimes are compared as instants (`cmpAsDatetimes`), so `.6` and
//!     `.600` are the same value, as are `-00:00` and `Z`;
//!   * booleans are compared case-insensitively;
//!   * everything else is compared as a string.
//!
//! Type tags must always match exactly.

mod common;

use common::{compare, value_to_json};
use serde_json::Value as JsonValue;
use std::fs;
use std::path::{Path, PathBuf};
use toml_rs_port::parse;

fn test_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("internal").join("toml-test").join("tests")
}

/// The corpus holds the TOML 1.0.0 and 1.1.0 suites side by side, and they
/// contradict each other: `valid/inline-table/newline.toml` requires accepting
/// a trailing comma, `invalid/inline-table/trailing-comma.toml` requires
/// rejecting it. The reference runner resolves this with a per-version
/// exclusion list (`internal/toml-test/version.go`); scoring against the raw
/// union of all 775 files is not a passable target for any implementation.
///
/// This port targets TOML 1.1.0, the same version the Go original tracks.
/// Override with `TOML_VERSION=1.0.0 cargo test --release`.
fn toml_version() -> String {
    std::env::var("TOML_VERSION").unwrap_or_else(|_| "1.1.0".to_string())
}

/// Mirrors `versions["1.1.0"].exclude` in internal/toml-test/version.go.
const EXCLUDE_1_1_0: &[&str] = &[
    "valid/spec-1.0.0/*",
    "invalid/spec-1.0.0/*",
    "invalid/datetime/no-secs",
    "invalid/local-time/no-secs",
    "invalid/local-datetime/no-secs",
    "invalid/string/basic-byte-escapes",
    "invalid/inline-table/trailing-comma",
    "invalid/inline-table/linebreak-01",
    "invalid/inline-table/linebreak-02",
    "invalid/inline-table/linebreak-03",
    "invalid/inline-table/linebreak-04",
];

/// Mirrors `versions["1.0.0"].exclude`.
const EXCLUDE_1_0_0: &[&str] = &[
    "valid/spec-1.1.0/*",
    "invalid/spec-1.1.0/*",
    "valid/string/escape-esc",
    "valid/string/hex-escape",
    "invalid/string/bad-hex-esc",
    "valid/datetime/no-seconds",
    "valid/inline-table/newline",
    "valid/inline-table/newline-comment",
    "invalid/control/multi-cr",
    "invalid/control/rawmulti-cr",
];

fn excluded_for(version: &str) -> &'static [&'static str] {
    match version {
        "1.0.0" => EXCLUDE_1_0_0,
        _ => EXCLUDE_1_1_0,
    }
}

/// Test name relative to the corpus root, without the `.toml` extension —
/// the form the exclusion patterns are written in.
fn test_name(path: &Path) -> String {
    path.strip_prefix(test_dir())
        .unwrap_or(path)
        .with_extension("")
        .to_string_lossy()
        .replace('\\', "/")
}

fn is_excluded(name: &str, version: &str) -> bool {
    excluded_for(version).iter().any(|pat| match pat.strip_suffix("/*") {
        Some(dir) => name.starts_with(dir) && name[dir.len()..].starts_with('/'),
        None => name == *pat,
    })
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
    files.sort();
    files
}

// ----- the tests -----------------------------------------------------------

/// Read a test file as UTF-8. `Err` means the bytes are not valid UTF-8 —
/// which, for an invalid-TOML test, is itself the expected rejection.
fn read_utf8(path: &Path) -> Result<String, ()> {
    String::from_utf8(fs::read(path).map_err(|_| ())?).map_err(|_| ())
}

#[test]
fn test_valid_toml_files() {
    let toml_files = find_toml_files(&test_dir().join("valid"));
    let version = toml_version();
    let mut passed = 0;
    let mut skipped = 0;
    let mut failures: Vec<String> = Vec::new();
    for path in &toml_files {
        let name = test_name(path);
        if is_excluded(&name, &version) { skipped += 1; continue; }
        let Ok(toml_content) = read_utf8(path) else {
            failures.push(format!("READ: {}", name));
            continue;
        };
        let expected: JsonValue = match fs::read_to_string(path.with_extension("json"))
            .ok()
            .and_then(|s| serde_json::from_str(&s).ok())
        {
            Some(j) => j,
            None => { failures.push(format!("NO EXPECTED JSON: {}", name)); continue; }
        };
        match parse(&toml_content) {
            Ok(value) => match compare(&expected, &value_to_json(&value)) {
                Ok(()) => passed += 1,
                Err(e) => failures.push(format!("MISMATCH: {} — {}", name, e)),
            },
            Err(e) => failures.push(format!("PARSE: {} — {}", name, e)),
        }
    }
    println!(
        "\n=== Valid (TOML {}): {} passed, {} failed, {} not in this version (of {} files) ===",
        version, passed, failures.len(), skipped, toml_files.len()
    );
    for f in failures.iter().take(20) { println!("  {}", f); }
    if failures.len() > 20 { println!("  ... +{} more", failures.len() - 20); }
    assert!(failures.is_empty(), "{} valid tests failed", failures.len());
}

/// Encoder conformance: every valid document must survive
/// parse → encode → parse and still match its expected JSON.
#[test]
fn test_encoder_round_trip() {
    let version = toml_version();
    let toml_files = find_toml_files(&test_dir().join("valid"));
    let mut passed = 0;
    let mut skipped = 0;
    let mut failures: Vec<String> = Vec::new();
    for path in &toml_files {
        let name = test_name(path);
        if is_excluded(&name, &version) { skipped += 1; continue; }
        let Ok(toml_content) = read_utf8(path) else { continue };
        let expected: JsonValue = match fs::read_to_string(path.with_extension("json"))
            .ok()
            .and_then(|s| serde_json::from_str(&s).ok())
        {
            Some(j) => j,
            None => continue,
        };
        let Ok(value) = parse(&toml_content) else { continue };
        let encoded = match toml_rs_port::encode(&value) {
            Ok(s) => s,
            Err(e) => { failures.push(format!("ENCODE: {} — {}", name, e)); continue; }
        };
        match parse(&encoded) {
            Ok(reparsed) => match compare(&expected, &value_to_json(&reparsed)) {
                Ok(()) => passed += 1,
                Err(e) => failures.push(format!("ROUND TRIP: {} — {}", name, e)),
            },
            Err(e) => failures.push(format!("REPARSE: {} — {}\n--- emitted ---\n{}", name, e, encoded)),
        }
    }
    println!(
        "\n=== Encoder round trip (TOML {}): {} passed, {} failed, {} not in this version ===",
        version, passed, failures.len(), skipped
    );
    for f in failures.iter().take(20) { println!("  {}", f); }
    if failures.len() > 20 { println!("  ... +{} more", failures.len() - 20); }
    assert!(failures.is_empty(), "{} encoder round trips failed", failures.len());
}

#[test]
fn test_invalid_toml_files() {
    let toml_files = find_toml_files(&test_dir().join("invalid"));
    let version = toml_version();
    let mut passed = 0;
    let mut skipped = 0;
    let mut failures: Vec<String> = Vec::new();
    for path in &toml_files {
        let name = test_name(path);
        if is_excluded(&name, &version) { skipped += 1; continue; }
        // Non-UTF-8 input is invalid TOML, so failing to decode it counts as
        // a correct rejection rather than a skipped test.
        let Ok(toml_content) = read_utf8(path) else { passed += 1; continue; };
        match parse(&toml_content) {
            Ok(_) => failures.push(format!("SHOULD ERROR: {}", name)),
            Err(_) => passed += 1,
        }
    }
    println!(
        "\n=== Invalid (TOML {}): {} passed, {} failed, {} not in this version (of {} files) ===",
        version, passed, failures.len(), skipped, toml_files.len()
    );
    for f in failures.iter().take(20) { println!("  {}", f); }
    if failures.len() > 20 { println!("  ... +{} more", failures.len() - 20); }
    assert!(failures.is_empty(), "{} invalid tests should have errored", failures.len());
}
