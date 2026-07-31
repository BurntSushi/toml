# Benchmark Methodology

## Environment
- **Machine:** TBD (will be filled at hackathon)
- **OS:** TBD
- **Rust version:** `rustc --version`
- **Go version:** `go version`
- **CPU:** TBD
- **RAM:** TBD

## Methodology

### Throughput
- Parse all 266 valid toml-test files repeatedly for 10 seconds
- Measure total bytes parsed / wall time
- Report MB/s

### p99 Latency
- Parse each valid toml-test file 10,000 times
- Record per-parse latency in microseconds
- Report p50, p90, p99, p99.9

### RSS (Resident Set Size)
- Run the decoder on the entire valid corpus
- Measure peak RSS via `/usr/bin/time -v` on Linux, `task_info` on macOS
- Report in MB

### Startup Time
- Measure cold-start time of the binary (no warm cache)
- Average over 100 runs
- Report in milliseconds

## Honesty Notes
- Both binaries compiled with `-O3` / `--release`
- No SIMD intrinsics used in either port (unless the original has them)
- p99 is reported, not just mean — per the hackathon's "honest numbers" requirement
- If the port is slower in any metric, it will be reported honestly

## Results

| Metric | Go Original | Rust Port | Delta | Notes |
|---|---|---|---|---|
| Throughput (MB/s) | TBD | TBD | TBD | |
| p50 latency (μs) | TBD | TBD | TBD | |
| p99 latency (μs) | TBD | TBD | TBD | |
| p99.9 latency (μs) | TBD | TBD | TBD | |
| Peak RSS (MB) | TBD | TBD | TBD | |
| Startup time (ms) | TBD | TBD | TBD | |