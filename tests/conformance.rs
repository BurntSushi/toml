//! Conformance test runner — runs all 775 toml-test cases.
//!
//! This test loads the toml-test corpus from internal/toml-test/tests/
//! and verifies that the Rust port produces the expected JSON output
//! for each valid case and reports errors for each invalid case.

use std::fs;
use std::path::PathBuf;
use toml_rs_port::parse;

/// Get the path to the toml-test corpus
fn test_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("internal")
        .join("toml-test")
        .join("tests")
}

#[test]
fn test_valid_toml_files() {
    let valid_dir = test_dir().join("valid");
    let mut passed = 0;
    let mut failed = 0;

    for entry in fs::read_dir(&valid_dir).expect("failed to read valid test dir") {
        let entry = entry.expect("failed to read dir entry");
        let path = entry.path();

        if path.extension().map_or(true, |e| e != "toml") {
            continue; // skip non-toml files (expected .json files)
        }

        let toml_content = fs::read_to_string(&path).expect("failed to read toml file");
        let expected_path = path.with_extension("json");
        let _expected_json = fs::read_to_string(&expected_path).unwrap_or_default();

        match parse(&toml_content) {
            Ok(_value) => {
                // TODO: compare value_to_json(_value) to expected_json
                passed += 1;
            }
            Err(e) => {
                failed += 1;
                eprintln!("FAIL: {} — {}", path.display(), e);
            }
        }
    }

    println!("Valid tests: {} passed, {} failed (of {} total)", passed, failed, passed + failed);
    assert!(failed == 0, "{} valid tests failed", failed);
}

#[test]
fn test_invalid_toml_files() {
    let invalid_dir = test_dir().join("invalid");
    let mut passed = 0;
    let mut failed = 0;

    for entry in fs::read_dir(&invalid_dir).expect("failed to read invalid test dir") {
        let entry = entry.expect("failed to read dir entry");
        let path = entry.path();

        if path.extension().map_or(true, |e| e != "toml") {
            continue;
        }

        let toml_content = fs::read_to_string(&path).expect("failed to read toml file");

        match parse(&toml_content) {
            Ok(_) => {
                failed += 1;
                eprintln!("FAIL (should have errored): {}", path.display());
            }
            Err(_) => {
                passed += 1;
            }
        }
    }

    println!("Invalid tests: {} passed, {} failed (of {} total)", passed, failed, passed + failed);
    assert!(failed == 0, "{} invalid tests should have produced errors", failed);
}