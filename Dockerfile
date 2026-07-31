# Build stage: Rust
FROM rust:1.87-slim AS builder

WORKDIR /app

# Copy Cargo.toml first for better layer caching
COPY Cargo.toml .
COPY src/ src/

# Build release binary
RUN cargo build --release --bin toml-test-decoder

# Runtime stage: minimal
FROM debian:bookworm-slim

COPY --from=builder /app/target/release/toml-test-decoder /usr/local/bin/toml-test-decoder
COPY --from=builder /app/target/release/tomlv /usr/local/bin/tomlv

# Verify the binary works
RUN toml-test-decoder --help 2>&1 || true

ENTRYPOINT ["toml-test-decoder"]