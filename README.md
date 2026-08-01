<div align="center">

# toml-rs

**Go to Rust | Port Mortem Hackathon | Track E**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Unsafe](https://img.shields.io/badge/unsafe-0-brightgreen.svg)](#)
[![Conformance](https://img.shields.io/badge/toml--test%201.1.0-710%2F710-brightgreen.svg)](#results)

</div>

Port of [BurntSushi/toml](https://github.com/BurntSushi/toml) (4,989 stars, ~4,900 LOC Go) to idiomatic Rust. Track E, Go to Rust.

| | |
|---|---|
| **Original** | [BurntSushi/toml](https://github.com/BurntSushi/toml) @ `c6d720d` |
| **Test corpus** | 775 toml-test conformance files |
| **Unsafe blocks** | 0 |
| **Core dependencies** | 0 (`serde_json` is used only by the test binaries) |
| **Test format** | Language-neutral: TOML stdin, JSON stdout |

## Results

The corpus is not a flat set of 775 tests. It holds the **TOML 1.0.0 and 1.1.0
suites side by side, and they contradict each other** — `valid/inline-table/newline.toml`
requires accepting a trailing comma in an inline table, while
`invalid/inline-table/trailing-comma.toml` requires rejecting it. The reference
runner resolves this with a per-version exclusion list
([`internal/toml-test/version.go`](internal/toml-test/version.go)); scoring
against the raw union is not a passable target for any implementation.

This port targets **TOML 1.1.0**, the same version the Go original tracks.

| Metric | Value |
|---|---|
| Valid documents (TOML 1.1.0) | **218 / 218** |
| Invalid documents rejected (TOML 1.1.0) | **492 / 492** |
| Encoder round trip (parse → encode → parse) | **218 / 218** |
| Divergences from the Go original on all 266 valid files | **0** |
| Documents Go accepts that this port correctly rejects | **18** |
| Test suite modifications | 0 |

Under the TOML 1.0.0 selection the port scores 209/209 valid and 490/499
invalid; the nine are exactly the 1.1.0 features it implements (optional
seconds, `\x` escapes, inline-table newlines and trailing commas), which the
Go original also accepts.

### Differential parity with the original

The Go original is kept in this repository and used as the oracle. On all 266
valid files, the port produces semantically identical output — every value
compared under toml-test's own rules (floats numerically, datetimes as
instants). There is no input in the corpus that this port accepts and the
original rejects.

### Bugs found in the original

The port rejects 18 documents that BurntSushi/toml accepts but the corpus marks
invalid — mostly around table state: redefining a table created by a dotted key,
extending an array of tables through a dotted key, and duplicate keys inside
inline tables. Full list in [DECISIONS.md](DECISIONS.md).

The differential fuzzer also found one bug in **this port**, since fixed: a float
literal whose exponent overflowed (`3.14159265358e9793`) silently became `inf`
instead of erroring.

## Build and Test

```bash
cargo build --release
cargo test --release                     # conformance + encoder round trip
TOML_VERSION=1.0.0 cargo test --release  # score against the 1.0.0 selection
make fuzz                                # 60s differential fuzz vs the Go original
```

The differential fuzzer needs a Go toolchain; it builds `cmd/toml-test-decoder`
once and shells out to it. Duration and seed are configurable:

```bash
FUZZ_SECONDS=300 FUZZ_SEED=42 cargo test --release --test fuzz -- --ignored --nocapture
```

## Why the Go sources are still here

They are not leftovers. `internal/toml-test/tests/` **is** the test corpus,
`cmd/toml-test-decoder` is the differential oracle, and `internal/toml-test/`
defines the comparison and version-selection rules the Rust harness mirrors.
Deleting them would remove both the tests and the reference.

## Structure

```
src/
  lib.rs              Value enum, parse(), encode()
  lex.rs              Lexer
  parse.rs            Parser, table-state tracking
  datetime.rs         RFC 3339 / TOML datetime grammar
  number.rs           Integer and float grammar, Go-compatible float formatting
  encode.rs           Encoder
  error.rs            Error types
  bin/
    toml_test_decoder.rs   toml-test wire protocol
    toml_test_encoder.rs   toml-test encoder protocol
    tomlv.rs               Validator CLI
tests/
  conformance.rs      Version-scoped runner over the toml-test corpus
  common/mod.rs       toml-test comparison rules, shared with the fuzzer
fuzz/
  harness.rs          Differential fuzzer against the Go original
bench/
  methodology.md      Benchmark methodology
DECISIONS.md          Architectural divergences
Dockerfile            One-command build
Makefile              DX shortcuts
.port-mortem.toml     Track, source URL, kickoff hash
```

## License

MIT (same as original)
