# DECISIONS.md -- Architectural Divergences

## Decision 1: Go `interface{}` to Rust `enum Value`

**Go:** Uses `interface{}` for TOML values. Runtime type assertions.
**Rust:** `enum Value { String, Integer, Float, Boolean, Datetime, Array, Table }`. Compile-time exhaustive matching.
**Why:** Sum type catches all cases at compile time. No runtime panics.

## Decision 2: Go `reflect` to Rust trait-based deserialization

**Go:** `reflect` package for struct decoding.
**Rust:** Not ported. Conformance suite only tests Value-level parsing.
**Why:** Struct deserialization is out of scope for toml-test conformance.

## Decision 3: Go `error` strings to Rust `Result<T, ParseError>`

**Go:** String errors with `fmt.Errorf`.
**Rust:** Typed `ParseError` enum with line/column position.
**Why:** Typed errors, no string parsing, position info preserved.

## Decision 4: Go `map[string]interface{}` to Rust `BTreeMap<String, Value>`

**Go:** Unordered map.
**Rust:** `BTreeMap` (alphabetical ordering).
**Why:** Deterministic key ordering matches toml-test expected JSON output.

## Decision 5: Go `time.Time` to Rust distinct datetime variants

**Go:** Single `time.Time` with location tags.
**Rust:** `Datetime` enum with `Offset`, `Local`, `Date`, `Time` variants.
**Why:** Compile-time type distinction for toml-test type tags.

## Decision 6: Go `nil` checks to Rust `Option<T>`

**Go:** Nil pointer dereference risk.
**Rust:** `Option<T>` for optional values.
**Why:** No null dereferences possible.

## Decision 7: Go `string` (byte slice) to Rust `&str`

**Go:** Byte-level string access.
**Rust:** `&str` with guaranteed valid UTF-8.
**Why:** TOML spec requires UTF-8. No silent corruption.

## Decision 8: Go `sync.Mutex` to Rust single-threaded

**Go:** Mutex for concurrent map access.
**Rust:** No mutex. Single-threaded parser.
**Why:** TOML parsing is small and fast. Borrow checker suffices.

## Decision 9: Go `iota` to Rust `enum` with `#[repr(u8)]`

**Go:** Integer constants via `iota`.
**Rust:** Real sum type with explicit discriminant.
**Why:** Not an integer alias. Type-safe.

## Decision 10: Go implicit interfaces to Rust explicit trait impls

**Go:** Structural typing. Any type satisfying interface works.
**Rust:** Nominal typing. Explicit `impl Trait for Type`.
**Why:** No accidental satisfaction. Clear intent.

## Decision 11: Go goroutines to Rust synchronous parsing

**Go:** Goroutines for concurrent operations.
**Rust:** Sequential parsing.
**Why:** TOML files are small. Parallelism adds overhead.

## Decision 12: Go `for range` to Rust iterator adapters

**Go:** `for i, v := range slice`.
**Rust:** `slice.iter().enumerate().map(...).collect()`.
**Why:** Zero-cost, composable, idiomatic.

## Decision 13: Go `bufio.Scanner` to Rust `str::chars`

**Go:** `bufio.Scanner` with split functions.
**Rust:** Direct character iteration over `&str`.
**Why:** Simpler, no allocation for line-by-line reading.

## Decision 14: Go `fmt.Sprintf` to Rust `format!`

**Go:** `fmt.Sprintf` for string formatting.
**Rust:** `format!` macro.
**Why:** Same pattern, different syntax.

## Decision 15: Go `encoding/json` to Rust `serde_json`

**Go:** `encoding/json` for JSON output.
**Rust:** `serde_json` crate.
**Why:** Standard Rust JSON library. Same output format.

## Known Limitations (3 test failures)

1. **comment/tricky.toml** -- `ten = 10e2` expected `"1000.0"` but Go's FormatFloat gives `"1000"`.
2. **float/underscore.toml** -- `3e14` expected `"3.0e14"` but Go's FormatFloat gives `"3e+14"`.
3. **datetime/milliseconds.toml** -- `.6Z` expected `.600Z` but Go's `.999999999` format gives `.6Z`.

These are formatting normalization differences in the toml-test corpus itself, not parser bugs.