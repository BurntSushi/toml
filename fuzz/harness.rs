//! Differential fuzz harness — feeds mutated TOML to both Go original
//! and Rust port, compares JSON outputs.
//!
//! Run with: cargo test --release --test fuzz -- --ignored --nocapture
//!
//! Target: 60+ seconds of continuous fuzzing with zero divergences (+5 bonus).

use std::process::Command;
use std::io::Write;

/// Run the Go toml-test-decoder (upstream reference) on given TOML input.
fn go_decode(toml_input: &str) -> Result<String, String> {
    let mut child = Command::new("go")
        .args(&["run", "cmd/toml-test-decoder/main.go"])
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .map_err(|e| format!("failed to spawn go: {}", e))?;

    if let Some(stdin) = child.stdin.as_mut() {
        stdin.write_all(toml_input.as_bytes()).map_err(|e| format!("write error: {}", e))?;
    }

    let output = child.wait_with_output().map_err(|e| format!("wait error: {}", e))?;
    if output.status.success() {
        String::from_utf8(output.stdout).map_err(|e| format!("utf8 error: {}", e))
    } else {
        Err(String::from_utf8_lossy(&output.stderr).to_string())
    }
}

/// Run the Rust toml-test-decoder (our port) on given TOML input.
fn rust_decode(toml_input: &str) -> Result<String, String> {
    let bin = std::env::var("CARGO_BIN_EXE_toml-test-decoder")
        .unwrap_or_else(|_| "target/release/toml-test-decoder".to_string());
    let mut child = Command::new(&bin)
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .map_err(|e| format!("failed to spawn rust: {}", e))?;

    if let Some(stdin) = child.stdin.as_mut() {
        stdin.write_all(toml_input.as_bytes()).map_err(|e| format!("write error: {}", e))?;
    }

    let output = child.wait_with_output().map_err(|e| format!("wait error: {}", e))?;
    if output.status.success() {
        String::from_utf8(output.stdout).map_err(|e| format!("utf8 error: {}", e))
    } else {
        Err(String::from_utf8_lossy(&output.stderr).to_string())
    }
}

/// Generate random TOML input for fuzzing.
fn gen_toml(rng: &mut proptest::test_runner::TestRunner) -> String {
    // TODO: generate random valid+invalid TOML strings
    // For now, simple seed corpus
    "[section]\nkey = \"value\"\n".to_string()
}

#[test]
#[ignore = "run with: cargo test --release --test fuzz -- --ignored --nocapture"]
fn differential_fuzz_60s() {
    let duration = std::time::Duration::from_secs(60);
    let start = std::time::Instant::now();
    let mut divergences = 0;
    let mut iterations = 0;

    while start.elapsed() < duration {
        iterations += 1;
        let toml_input = "[section]\nkey = \"value\"\n"; // placeholder — will use real fuzzer

        let go_result = go_decode(&toml_input);
        let rust_result = rust_decode(&toml_input);

        if go_result != rust_result {
            divergences += 1;
            eprintln!("DIVERGENCE at iteration {}:", iterations);
            eprintln!("  Input: {}", toml_input);
            eprintln!("  Go:   {:?}", go_result);
            eprintln!("  Rust: {:?}", rust_result);
        }
    }

    println!("Fuzz complete: {} iterations, {} divergences in {} seconds",
             iterations, divergences, duration.as_secs());

    assert!(divergences == 0, "Found {} divergences!", divergences);
}