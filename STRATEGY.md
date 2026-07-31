# Port Mortem Hackathon — Full Strategy & Documentation

> This document captures EVERYTHING: the hackathon theme, our strategy, the wins,
> the blockers, the problems, and the AI prompts used for porting.
> It serves as the "write-up" for the $300 Write-Up side quest.

---

## 1. THE HACKATHON THEME

### What Port Mortem asks
Pick a real public GitHub repo in language X. Rewrite it in language Y. Prove the
original test suite still passes, unmodified. Survive a differential fuzz session.
Show your unsafe count. Defend your architectural decisions to a human.

### The Bun Scandal (the theme's inspiration)
In May 2026, Bun merged a 960,000-line, 2,188-file rewrite from Zig to Rust in 6 days,
generated mostly by 64 Claude Code agents. At merge it had a 99.8% Linux x64 test-pass
rate — and **13,044 unsafe blocks** (for comparison, Astral's uv has 73). Then people
noticed parts of the original Bun test suite had been edited to make the Rust port green.

The "rewrite it in Rust" meme has officially graduated to corporate strategy:
- Microsoft's goal: port 1B lines of C/C++ to Rust by 2030, AI-assisted
- DARPA's TRACTOR program: funding the same direction
- Discord's Go-to-Rust migration: still cited in every distributed-systems hiring loop
- Astral has rebuilt the entire Python tooling stack (ruff, uv, ty) in Rust
- Cloudflare's Pingora replaced NGINX with Rust at 40M+ req/sec

**The gap:** Generating ports is solved. **Proving they work** is the open problem.

### Scoring
| Criterion | Weight | What it covers |
|---|---|---|
| Functionality & Reliability | 40% | Test parity, unmodified, one-command build |
| Behavioral Equivalence | 30% | Differential fuzz, p99/RSS/startup, soak test |
| Code Quality | 20% | Idiomatic, unsafe ratio, decision log |
| Innovation | 10% | Creative pair, bug caught, upstream-mergeable |

### Bonuses
| Bonus | Points | Requirement |
|---|---|---|
| Differential Fuzz Survivor | +5 | 60s+ zero divergences on shared public API |
| Zero Unsafe | +5 | unsafe/any block count under threshold |
| Bug Catcher | +3 | Discover latent bug in original, file upstream issue |
| Decision Log | +3 | 10+ non-trivial architectural divergences with rationale |

### Side Quest
- **The Write-Up ($300 × 3)**: Publish a post about the port — the debugging story,
  the disappointing benchmark, the unsafe block you couldn't remove.
  Judged on insight, not follower count.

---

## 2. OUR SELECTION: BurntSushi/toml (Go → Rust)

### Why This Repo
- **4,989 stars** — top 5 most popular in the 104-repo pool
- **~4,000 core LOC** — solidly in the 2k-8k sweet spot
- **775 toml-test conformance files** (266 valid + 509 invalid) bundled in-repo
- **toml-test is a LANGUAGE-NEUTRAL protocol**: TOML stdin → JSON stdout
- **ZERO adapter complexity**: build a Rust binary, run the corpus against it
- **Written by BurntSushi** (ripgrep author) — high credibility
- **Zero external Go dependencies** (stdlib only)
- **Existing ossfuzz harness** for differential fuzzing

### Why NOT Other Repos (evaluated and rejected)
| Repo | Why rejected |
|---|---|
| textdistance (Py→Rust) | Tests import Python module, pass Python callables (sim_func), call internal methods. Requires PyO3 adapter (complex, rule-gray-area). ~25 tests can't be served by binary. |
| decimal.js (JS→Rust) | 185 global config state changes between tests. Expected outputs depend on config state that mutates throughout test files. Adapter is HIGH complexity, not LOW as initially thought. |
| cJSON (C→Rust) | Only 14 file-based tests are binary-testable; 483 tests call internal C functions needing C-ABI adapter. |
| tinyexpr (C→Rust) | Only 821 LOC — too small for "harder" requirement. |
| mustache.js (JS→Rust) | Only 764 LOC — too small. |
| bignumber.js (JS→Rust) | 445 global config state changes — same problem as decimal.js. |
| natsort (Py→Rust) | Tests import Python module, same adapter problem as textdistance. |

---

## 3. THE WINS (what makes this a winning strategy)

### Win 1: The Independent Oracle
The toml-test corpus is THE canonical cross-language TOML conformance protocol.
It's language-neutral (TOML in → JSON out), bundled in-repo, and predates our port.
This is exactly the "independent test suite" pattern the Bun story praised.
Judges will recognize this immediately.

### Win 2: Zero Adapter Complexity
Unlike textdistance (needs PyO3), cJSON (needs C-ABI), or decimal.js (needs config
state machine replay), our test adapter is ZERO complexity:
1. Build a Rust binary that reads TOML from stdin
2. Output JSON to stdout
3. Run all 775 test cases: `cat test.toml | toml-test-decoder`
4. Compare JSON output to expected JSON

No language runtime, no FFI, no config state, no callables.

### Win 3: Zero Unsafe — Guaranteed by Problem Domain
TOML parsing is pure string/number/table manipulation. No FFI, no memory
manipulation, no raw pointers. Zero unsafe blocks is not just achievable — it's
the NATURAL state of the port. This contrasts dramatically with Bun's 13,044.

### Win 4: The "Discord Canon" Narrative
Track E is "The Discord canon" — GC pauses vs. predictable tail latency.
Go's garbage collector introduces tail latency; Rust's ownership model eliminates it.
This is the exact story Discord published about their read-states service migration.
Judges from AWS, Google, Oracle will appreciate this.

### Win 5: All 4 Bonuses Achievable
- **+5 Differential Fuzz**: ossfuzz harness already exists; feed mutated TOML to
  Go original + Rust binary, compare JSON
- **+5 Zero Unsafe**: pure parsing, no FFI
- **+3 Bug Catcher**: TOML parsing has edge cases (datetime formats, multi-line
  strings, array nesting, key duplication, unicode in keys)
- **+3 Decision Log**: 15+ architectural decisions documented (see DECISIONS.md)

### Win 6: Judge Alignment
- **Deep Saxena (Microsoft Sr SWE/TL)**: Parser/compiler work — CDAC systems pedigree
- **Tabby/Refact.ai**: Pure Rust, zero unsafe, 775 conformance tests = unimpeachable
- **AWS trio**: One-command build, deterministic tests, clean CLI
- **Coinbase/Zscaler**: Config parsing correctness is security-critical (Cargo.toml!)
- **Enterprise panel**: 4,989 stars, BurntSushi pedigree, "I'd merge this upstream"

---

## 4. THE BLOCKERS (risks and mitigations)

### Blocker 1: Datetime Handling
**Risk**: Go's `time.Time` handles all datetime variants (offset, local, date, time).
The toml-test protocol encodes datetime types explicitly. Rust doesn't have a
std datetime type — we need custom datetime types.
**Mitigation**: Use the `Datetime` enum in `lib.rs` with distinct variants for each type.
The toml-test JSON format tags types explicitly.

### Blocker 2: Reflection-Based Decoding
**Risk**: The Go original uses `reflect` to decode into arbitrary Go structs.
The toml-test conformance suite only tests Value-level decoding (not struct decoding),
but some Go unit tests do test struct decoding.
**Mitigation**: Port only the Value-level parser for the conformance suite.
Document struct deserialization as out-of-scope in DECISIONS.md.

### Blocker 3: Float Formatting
**Risk**: The toml-test protocol expects specific float formatting (e.g., `42.0` not `42`).
Rust's `f64::to_string()` may format differently from Go's `fmt.Sprintf`.
**Mitigation**: Custom `format_float()` function in the decoder binary to match
Go's formatting exactly.

### Blocker 4: Table Key Ordering
**Risk**: Go maps are unordered; our BTreeMap is alphabetically ordered.
The toml-test expected JSON files may assume a specific key order.
**Mitigation**: BTreeMap gives deterministic alphabetical ordering — if the
expected JSON is also alphabetical (it is, per toml-test spec), this is fine.

### Blocker 5: Error Message Format
**Risk**: The toml-test protocol for invalid tests just checks that the decoder
reports an error (non-zero exit). But our error messages differ from Go's.
**Mitigation**: For invalid tests, we just need to exit non-zero. The error
message format doesn't need to match Go's for the conformance suite.

### Blocker 6: Multi-Line Strings
**Risk**: TOML supports multi-line basic strings (`"""..."""`) and multi-line
literal strings (`'''...'''`). The lexer needs to handle these.
**Mitigation**: Implement multi-line string lexing in `lex.rs`.

### Blocker 7: Array of Tables (`[[...]]`)
**Risk**: TOML's `[[table]]` syntax creates arrays of tables. The parser needs
to handle this correctly.
**Mitigation**: Implement array-of-tables handling in `parse.rs`.

---

## 5. THE PROBLEMS (what could go wrong during 72h)

### Problem 1: Time Budget
72 hours for ~4,000 LOC port + test adapter + fuzzing + benchmarks + docs.
With AI assistance, the porting itself is feasible, but the full submission
package (DECISIONS.md, benchmarks, fuzzing, demo video) takes significant time.
**Plan**: Allocate 48h to porting+testing, 12h to fuzzing+benchmarks, 12h to docs+video.

### Problem 2: toml-test JSON Format Matching
The toml-test protocol has specific JSON format requirements (type-tagged values).
Getting the JSON output exactly right for all 266 valid tests is fiddly.
**Plan**: Study the expected JSON files early and build `value_to_json()` carefully.

### Problem 3: Track E Competition
Track E is "Medium" difficulty — more competition than Track A (Hard).
**Plan**: The 775-test conformance + zero unsafe + all bonuses makes us stand out.

### Problem 4: Go Build for Differential Fuzzing
Differential fuzzing requires running the Go original alongside the Rust port.
We need the Go toolchain installed and the Go decoder binary built.
**Plan**: Build the Go reference binary early: `go build cmd/toml-test-decoder/`

---

## 6. AI PROMPTS USED FOR PORTING

These are the prompts fed to AI coding agents (Cursor, Claude Code, etc.) to port
each component. Each prompt is designed to produce idiomatic Rust from the Go original.

### Prompt 1: Lexer (lex.go → lex.rs)
```
You are porting a Go TOML lexer to Rust. The Go file is lex.go (1248 LOC).

Read the Go source at: lex.go
Port it to idiomatic Rust with these requirements:
1. Replace Go's bufio.Scanner with Rust char_indices() iteration
2. Replace Go's interface{} return with a Token enum
3. Replace Go's error handling (err != nil) with Result<Token, ParseError>
4. Replace Go's switch with Rust match
5. Track byte positions for error messages (line, column)
6. Handle UTF-8 correctly — Rust &str guarantees valid UTF-8
7. ZERO unsafe blocks — this is pure string processing
8. Keep the same token types as the original

Output: src/lex.rs
```

### Prompt 2: Parser (parse.go → parse.rs)
```
You are porting a Go TOML parser to Rust. The Go file is parse.go (846 LOC).

Read the Go source at: parse.go
Port it to idiomatic Rust with these requirements:
1. Replace Go's interface{} with a Value enum (String, Integer, Float, Boolean, Datetime, Array, Table)
2. Replace Go's recursive descent with a Parser struct holding token stream
3. Replace Go's map[string]interface{} with BTreeMap<String, Value> (deterministic ordering)
4. Replace Go's error handling with Result<Value, ParseError>
5. Handle dotted keys (a.b.c = value) by creating nested tables
6. Handle table headers ([a.b.c]) and array-of-tables ([[a.b.c]])
7. Handle inline tables ({ key = value, ... })
8. ZERO unsafe blocks
9. ZERO external dependencies (std only)

Output: src/parse.rs
```

### Prompt 3: Encoder (encode.go → encode.rs)
```
You are porting a Go TOML encoder to Rust. The Go file is encode.go (784 LOC).

Read the Go source at: encode.go
Port it to idiomatic Rust with these requirements:
1. Encode Value enum back to TOML string
2. Handle string escaping (basic strings with \" \\ \n \t \r \uXXXX)
3. Handle multi-line strings where appropriate
4. Handle float formatting (match Go's strconv.FormatFloat)
5. Handle table formatting (key = value pairs, one per line)
6. Handle array formatting (inline or multi-line for large arrays)
7. ZERO unsafe blocks

Output: src/encode.rs
```

### Prompt 4: toml-test-decoder Binary
```
Create a Rust binary that implements the toml-test wire protocol.

Requirements:
1. Read TOML from stdin
2. Parse it using our parse() function
3. Convert the Value tree to toml-test JSON format:
   - Strings: {"type": "string", "value": "..."}
   - Integers: {"type": "integer", "value": "42"}
   - Floats: {"type": "float", "value": "3.14"}
   - Booleans: {"type": "bool", "value": "true"}
   - Datetimes: {"type": "datetime", "value": "..."}
   - Arrays: [wrapped_value, ...]
   - Tables: {"key": wrapped_value, ...}
4. Output JSON to stdout
5. On parse error: output error to stderr, exit non-zero

Output: src/bin/toml_test_decoder.rs
```

### Prompt 5: Differential Fuzz Harness
```
Create a differential fuzz harness that:
1. Generates random TOML input (valid and invalid)
2. Feeds it to both the Go original (cmd/toml-test-decoder) and the Rust port
3. Compares JSON outputs (normalized — whitespace-insensitive)
4. Reports any divergences
5. Runs for 60+ seconds continuously
6. Counts total iterations and divergences

The harness should use proptest for input generation.
Output: fuzz/harness.rs
```

### Prompt 6: Conformance Test Runner
```
Create a test that:
1. Reads all 266 valid .toml files from internal/toml-test/tests/valid/
2. Parses each with our parse() function
3. Converts the result to toml-test JSON format
4. Compares to the corresponding .json expected output file
5. Reports pass/fail counts
6. Also reads all 509 invalid .toml files from internal/toml-test/tests/invalid/
7. Verifies that each invalid file produces a parse error

Output: tests/conformance.rs
```

### Prompt 7: Benchmark Suite
```
Create a benchmark that:
1. Parses all 266 valid toml-test files repeatedly
2. Measures: throughput (MB/s), p50/p90/p99/p99.9 latency, peak RSS, startup time
3. Compares Go original vs Rust port
4. Outputs results to bench/results.json
5. Reports honestly — if the port is slower, say so

Use std::time::Instant for timing.
Output: bench/bench.rs
```

---

## 7. EXECUTION TIMELINE (72 HOURS)

| Timeframe | Task | AI Prompt # |
|---|---|---|
| **Hour 0–2** | Fork repo, set up project structure, pin test hashes | — |
| **Hour 2–12** | Port lexer (lex.go → lex.rs) | Prompt 1 |
| **Hour 12–24** | Port parser (parse.go → parse.rs) | Prompt 2 |
| **Hour 24–32** | Port encoder (encode.go → encode.rs) | Prompt 3 |
| **Hour 32–36** | Build toml-test-decoder binary + value_to_json | Prompt 4 |
| **Hour 36–44** | Run conformance suite, fix failures | Prompt 6 |
| **Hour 44–52** | Port remaining (decode.go, error.go, meta.go, types.go) | — |
| **Hour 52–60** | Build differential fuzz harness, run 60s+ | Prompt 5 |
| **Hour 60–66** | Benchmarks (p99, RSS, startup) + methodology | Prompt 7 |
| **Hour 66–70** | Write DECISIONS.md, README, demo video | — |
| **Hour 70–72** | Final commit, push, submit | — |

---

## 8. DEMO VIDEO SCRIPT (5 minutes)

1. **(0:00-0:30)** Title: "Port Mortem: BurntSushi/toml — Go to Rust"
2. **(0:30-1:00)** The repo: 4,989 stars, 4,000 LOC, BurntSushi, toml-test conformance
3. **(1:00-2:00)** Build: `cargo build --release` — one command, zero external deps
4. **(2:00-3:30)** Tests passing live: `cargo test --release` — 775/775 conformance tests
5. **(3:30-4:00)** Zero unsafe: `grep -r unsafe src/ | wc -l` → 0
6. **(4:00-4:30)** Differential fuzz: 60s+ run, zero divergences
7. **(4:30-5:00)** Benchmark comparison: Go vs Rust — p99, RSS, startup

---

## 9. WRITE-UP (for $300 side quest)

> "Generating a port is trivial. Explaining how you proved it holds up is the part
> almost nobody does."

Our write-up will cover:
- Why we chose toml (the independent oracle pattern)
- The edge case that ate 6 hours (likely: datetime formatting in toml-test JSON)
- The disappointing benchmark (likely: Rust not faster than Go for small files)
- The unsafe block we couldn't remove (spoiler: there were none — pure parsing)
- The architectural decision we'd take back (likely: BTreeMap vs IndexMap for key ordering)

Published on: Dev.to or LinkedIn (tagged with #PortMortem #HackathonRaptors)