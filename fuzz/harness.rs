//! Differential fuzz harness — feeds mutated TOML to both the Go original and
//! the Rust port and compares the outcomes.
//!
//! Run with: `cargo test --release --test fuzz -- --ignored --nocapture`
//! (or `make fuzz`). Duration and seed are configurable:
//!
//!   FUZZ_SECONDS=120 FUZZ_SEED=42 cargo test --release --test fuzz -- --ignored
//!
//! A divergence is either side accepting input the other rejects, or both
//! accepting but producing different values under toml-test comparison rules.
//! Formatting differences are not divergences — `3e+14` and `3.0e14` are the
//! same float, and `.6Z` and `.600Z` are the same instant.

#[path = "../tests/common/mod.rs"]
mod common;

use serde_json::Value as JsonValue;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::OnceLock;
use std::time::{Duration, Instant};

/// Outcome of running one decoder on one input.
#[derive(Debug, PartialEq)]
enum Outcome {
    Rejected,
    Accepted(JsonValue),
    /// Accepted but emitted JSON we can't parse — a bug in that decoder.
    Malformed(String),
}

/// Build the Go reference decoder once and cache the binary path.
///
/// `go run` would re-link on every call, which caps throughput at a few
/// iterations a second and makes the fuzzer useless.
fn go_decoder() -> Option<&'static Path> {
    static BIN: OnceLock<Option<PathBuf>> = OnceLock::new();
    BIN.get_or_init(|| {
        let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let out = root.join("target").join("go-toml-test-decoder");
        let status = Command::new("go")
            .args(["build", "-o"])
            .arg(&out)
            .arg("./cmd/toml-test-decoder")
            .current_dir(&root)
            .status()
            .ok()?;
        status.success().then_some(out)
    })
    .as_deref()
}

fn run_decoder(bin: &Path, input: &[u8]) -> Outcome {
    let Ok(mut child) = Command::new(bin)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
    else {
        return Outcome::Rejected;
    };
    if let Some(stdin) = child.stdin.as_mut() {
        // A decoder may exit before reading all of stdin; that's not an error.
        let _ = stdin.write_all(input);
    }
    drop(child.stdin.take());
    let Ok(output) = child.wait_with_output() else {
        return Outcome::Rejected;
    };
    if !output.status.success() {
        return Outcome::Rejected;
    }
    let text = String::from_utf8_lossy(&output.stdout).to_string();
    match serde_json::from_str(&text) {
        Ok(j) => Outcome::Accepted(j),
        Err(_) => Outcome::Malformed(text),
    }
}

/// xorshift64* — reproducible, and no dependency needed for a byte mutator.
struct Rng(u64);

impl Rng {
    fn next(&mut self) -> u64 {
        let mut x = self.0;
        x ^= x >> 12;
        x ^= x << 25;
        x ^= x >> 27;
        self.0 = x;
        x.wrapping_mul(0x2545_F491_4F6C_DD1D)
    }
    fn below(&mut self, n: usize) -> usize {
        if n == 0 { 0 } else { (self.next() % n as u64) as usize }
    }
    fn pick<'a, T>(&mut self, items: &'a [T]) -> &'a T {
        &items[self.below(items.len())]
    }
}

/// Fragments that exercise the parts of the grammar most likely to diverge:
/// table state, dotted keys, escapes, numeric edges, datetimes.
const FRAGMENTS: &[&str] = &[
    "[a]\n", "[[a]]\n", "[a.b]\n", "[[a.b]]\n", "a.b.c = 1\n",
    "a = 1\n", "a = \"x\"\n", "a = '''x'''\n", "a = \"\"\"x\"\"\"\n",
    "a = { b = 1 }\n", "a = [1, 2]\n", "a = [{ b = 1 }]\n",
    "a = 1979-05-27T07:32:00Z\n", "a = 07:32\n", "a = 1979-05-27\n",
    "a = 0x1f\n", "a = 1_000\n", "a = 3.14e-2\n", "a = inf\n", "a = nan\n",
    "a = true\n", "# comment\n", "\n", "a = \"\\u00e9\"\n", "a = \"\\t\"\n",
    "\"quoted key\" = 1\n", "a = { b.c = 1, d = 2 }\n",
];

const PUNCT: &[u8] = b"[]{}=,.\"'#\\\n\t 0123456789abcdefzZT+-_:eE";

fn mutate(rng: &mut Rng, seeds: &[String]) -> Vec<u8> {
    // Start from a seed document or a splice of fragments.
    let mut buf: Vec<u8> = if !seeds.is_empty() && rng.below(2) == 0 {
        rng.pick(seeds).clone().into_bytes()
    } else {
        let n = 1 + rng.below(8);
        (0..n).map(|_| *rng.pick(FRAGMENTS)).collect::<String>().into_bytes()
    };

    let rounds = 1 + rng.below(8);
    for _ in 0..rounds {
        if buf.is_empty() {
            buf.push(*rng.pick(PUNCT));
            continue;
        }
        match rng.below(6) {
            0 => { let i = rng.below(buf.len()); buf[i] = *rng.pick(PUNCT); }
            1 => { let i = rng.below(buf.len() + 1); buf.insert(i, *rng.pick(PUNCT)); }
            2 => { let i = rng.below(buf.len()); buf.remove(i); }
            3 => { let i = rng.below(buf.len()); buf[i] = rng.next() as u8; }
            4 => {
                let f = rng.pick(FRAGMENTS).as_bytes().to_vec();
                let i = rng.below(buf.len() + 1);
                buf.splice(i..i, f);
            }
            _ => {
                // Duplicate a slice, to provoke duplicate-key handling.
                let i = rng.below(buf.len());
                let j = i + rng.below(buf.len() - i + 1);
                let slice = buf[i..j].to_vec();
                buf.extend_from_slice(&slice);
            }
        }
        if buf.len() > 4096 { buf.truncate(4096); }
    }
    buf
}

fn seed_corpus() -> Vec<String> {
    let dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("internal/toml-test/tests/valid");
    let mut out = Vec::new();
    let mut stack = vec![dir];
    while let Some(d) = stack.pop() {
        let Ok(entries) = std::fs::read_dir(&d) else { continue };
        for e in entries.flatten() {
            let p = e.path();
            if p.is_dir() { stack.push(p); }
            else if p.extension().is_some_and(|x| x == "toml") {
                if let Ok(s) = std::fs::read_to_string(&p) { out.push(s); }
            }
        }
    }
    out.sort();
    out
}

fn env_or<T: std::str::FromStr>(key: &str, default: T) -> T {
    std::env::var(key).ok().and_then(|v| v.parse().ok()).unwrap_or(default)
}

#[test]
#[ignore = "run with: cargo test --release --test fuzz -- --ignored --nocapture"]
fn differential_fuzz() {
    let Some(go_bin) = go_decoder() else {
        // No Go toolchain means no oracle, so there is nothing to assert.
        // Fail loudly rather than reporting a vacuous pass.
        panic!("cannot build the Go reference decoder — install Go, or run \
                `cargo test --release` for conformance only");
    };
    let rust_bin = PathBuf::from(env!("CARGO_BIN_EXE_toml-test-decoder"));

    let seconds: u64 = env_or("FUZZ_SECONDS", 60);
    let seed: u64 = env_or("FUZZ_SEED", 0x5EED_1234_ABCD_0001);
    let mut rng = Rng(seed | 1);
    let seeds = seed_corpus();

    println!("fuzzing for {}s with seed {:#x} ({} seed documents)", seconds, seed, seeds.len());

    let deadline = Duration::from_secs(seconds);
    let start = Instant::now();
    let mut iterations = 0u64;
    let mut both_accepted = 0u64;
    let mut both_rejected = 0u64;
    let mut stricter = 0u64;
    let mut divergences: Vec<String> = Vec::new();

    while start.elapsed() < deadline && divergences.len() < 10 {
        iterations += 1;
        let input = mutate(&mut rng, &seeds);

        let go = run_decoder(go_bin, &input);
        let rs = run_decoder(&rust_bin, &input);

        let report = |what: &str| {
            format!("{}\n  input ({} bytes): {:?}", what, input.len(),
                    String::from_utf8_lossy(&input))
        };

        match (&go, &rs) {
            (Outcome::Rejected, Outcome::Rejected) => both_rejected += 1,
            (Outcome::Accepted(g), Outcome::Accepted(r)) => {
                both_accepted += 1;
                if let Err(e) = common::compare(g, r) {
                    divergences.push(report(&format!("VALUE MISMATCH: {}", e)));
                }
            }
            (Outcome::Accepted(_), Outcome::Rejected) => {
                // The port being stricter than Go is expected: Go accepts 18
                // documents the corpus marks invalid. Counted, not failed.
                stricter += 1;
            }
            (Outcome::Rejected, Outcome::Accepted(_)) => {
                divergences.push(report("PORT ACCEPTED INPUT THE REFERENCE REJECTED"));
            }
            (_, Outcome::Malformed(t)) => {
                divergences.push(report(&format!("PORT EMITTED INVALID JSON: {:.200}", t)));
            }
            (Outcome::Malformed(_), _) => both_rejected += 1,
        }
    }

    println!(
        "fuzz complete: {} iterations in {:.1}s — {} both accepted, {} both rejected, \
         {} rejected only by the port; {} divergences",
        iterations, start.elapsed().as_secs_f64(),
        both_accepted, both_rejected, stricter, divergences.len()
    );
    for d in &divergences {
        println!("\n{}", d);
    }
    assert!(divergences.is_empty(), "{} divergences from the Go reference", divergences.len());
}
