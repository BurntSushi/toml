# Benchmark Methodology

## Metrics

p99 latency, peak RSS, startup time, throughput. Go original vs Rust port.

## Method

Parse all 266 valid toml-test files. Measure with std::time::Instant. Report honestly.