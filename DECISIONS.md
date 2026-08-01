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

## Decision 16: Go's permissive `time.Parse` layouts to an explicit datetime grammar

**Go:** `time.Parse` with a table of layout strings, which accepts whatever the
layout happens to match — `1987-7-05T17:45:00Z` slips through with a one-digit month.
**Rust:** `src/datetime.rs` walks the RFC 3339 / TOML ABNF byte by byte.
**Why:** Every rejection has a stated reason, and the accept set is exactly the
spec's rather than an artefact of a format string.

## Decision 17: Go's `strconv` fallback to an explicit number grammar

**Go:** Lex loosely, then hand the token to `strconv`, which accepts things TOML does
not (`1.e2`, `_1.2`, `0x_1`).
**Rust:** `src/number.rs` validates against the ABNF before conversion.
**Why:** Same reason. Underscore placement, leading zeros, and the
`frac`/`exp` structure are all grammar rules, so they belong in the grammar.

One behaviour is deliberately preserved from Go: a float literal that
*overflows* is an error, but one that *underflows* to zero is accepted. Rust's
`str::parse::<f64>` returns `inf` on overflow rather than erroring, so the check
is explicit.

## Decision 18: Go's `implicit` flag to a six-way table provenance enum

**Go:** Table nodes carry a single `implicit` boolean.
**Rust:** `Kind` in `src/parse.rs` records *how* each path came to exist:
`Header`, `Implicit`, `Aot`, `Dotted`, `Inline`, `InlineDotted`, `Value`, keyed by a
canonical path with array-of-table indices baked in.
**Why:** Every "already defined" rule in the spec becomes a single lookup. This is
what a boolean cannot express, and it is the source of most of the 18 bugs listed
below.

## Decision 19: Type tagging belongs in the library, not the test harness

**Go:** The decoder knows a `time.Time` from a `string`, so the distinction survives.
**Rust:** `Value::Datetime(Datetime)` is a real variant, and `Value::type_tag()`
lives in `src/lib.rs`.
**Why:** An earlier revision of this port stored datetimes as `Value::String` and had
the test harness re-derive the type by pattern-matching the text. That reported the
*quoted string* `"1979-05-27"` as a `date-local`. The corpus does not happen to cover
that case, so it passed. Type information must not be reconstructed downstream.

## Bugs found in BurntSushi/toml

The port rejects 18 documents that the original accepts and the corpus marks
invalid. Verified by running `cmd/toml-test-decoder` (the original, built from
this repository at the kickoff commit) over `internal/toml-test/tests/invalid`.

Table state — a table created by a dotted key, or an array of tables, being
reopened or extended in a way the spec forbids:

- `table/append-with-dotted-keys-01`, `-02`, `-03`, `-05`, `-08`
- `table/duplicate-key-04`, `-05`
- `table/redefine-02`, `-03`
- `array/extend-defined-aot`
- `spec-1.0.0/table-9-1`, `spec-1.1.0/common-46-1`, `spec-1.1.0/common-49-0`

Inline tables — duplicate and overwriting keys:

- `inline-table/duplicate-key-03`, `inline-table/overwrite-02`, `-08`
- `spec-1.0.0/inline-table-2-0`

Datetime range checking:

- `datetime/offset-overflow-minute`

There is no input in the corpus that this port accepts and the original rejects.

## Bugs the differential fuzzer found in this port

Three, all cases where this port accepted input the original rejects, and none
of them covered by the conformance corpus.

1. **Float overflow became `inf`.** `3.14159265358e9793` was accepted, because
   Rust's `str::parse::<f64>` saturates on overflow where Go's
   `strconv.ParseFloat` returns a range error. Fixed in `src/number.rs`; see
   Decision 17.
2. **A bare CR could start a line continuation.** In `"""\<CR>T"""` the
   backslash-newline handler treated a lone carriage return as a line ending
   and swallowed it. A CR only ends a line as part of CRLF. Fixed in
   `lex_multi`.
3. **Keys were lexed as values inside an inline table nested in an array.**
   `[{a = 1}, {b = 2}]` leaves an array and an inline table open at the same
   time, and the lexer decided what a comma meant from a bracket-depth counter,
   so after the second `{` it read `la+t_name` as a value token and accepted an
   illegal bare key. `is_value_position` now tracks enclosing containers as a
   stack and asks the innermost one.

## Corpus note: 775 tests is not a scoreable total

`internal/toml-test/version.go` defines per-version exclusion lists because the
TOML 1.0.0 and 1.1.0 suites in this corpus contradict each other —
`valid/inline-table/newline.toml` requires accepting a trailing comma in an
inline table and `invalid/inline-table/trailing-comma.toml` requires rejecting
it. No implementation can pass both. `tests/conformance.rs` mirrors those lists
and defaults to TOML 1.1.0, matching the Go original's own behaviour.