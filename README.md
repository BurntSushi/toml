<div align="center">

# toml-rs

**Go to Rust | Port Mortem Hackathon | Track E**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Unsafe](https://img.shields.io/badge/unsafe-0-brightgreen.svg)](#)

</div>

Port of [BurntSushi/toml](https://github.com/BurntSushi/toml) (4,989 stars, ~4,000 LOC Go) to idiomatic Rust. Track E, Go to Rust.

| | |
|---|---|
| **Original** | [BurntSushi/toml](https://github.com/BurntSushi/toml) |
| **Test corpus** | 775 toml-test conformance files (266 valid + 509 invalid) |
| **Unsafe blocks** | 0 |
| **Test format** | Language-neutral: TOML stdin, JSON stdout |

## Results

| Metric | Value |
|---|---|
| Valid TOML tests | 263 / 266 |
| Invalid TOML tests | 319 / 500 |
| Test suite modifications | 0 |

## Build and Test

```bash
cargo build --release
cargo test --release
```

## Structure

```
src/
  lib.rs              Value enum, parse(), encode()
  lex.rs              Lexer
  parse.rs            Parser
  encode.rs           Encoder
  error.rs            Error types
  bin/
    toml_test_decoder.rs   toml-test wire protocol
    toml_test_encoder.rs   toml-test encoder
    tomlv.rs               Validator CLI
tests/
  conformance.rs      Runs all 775 toml-test cases
fuzz/
  harness.rs           Differential fuzzer
bench/
  methodology.md       Benchmark methodology
DECISIONS.md           Architectural divergences
Dockerfile             One-command build
Makefile               DX shortcuts
.port-mortem.toml      Track, source URL, kickoff hash
```

## License

MIT (same as original)