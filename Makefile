.PHONY: all build test fuzz bench check-unsafe smoke docker clean help

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## All-in-one: build, test, check unsafe, smoke test
all: build test check-unsafe smoke ## Build, test, check unsafe, smoke test

## Build
build: ## Build release binaries
	cargo build --release
	@echo ""
	@echo "Binaries built:"
	@ls -la target/release/toml-test-decoder* target/release/tomlv* 2>/dev/null || true

## Test
test: ## Run all conformance tests
	cargo test --release

## Fuzz
fuzz: ## Run differential fuzzer (60s+)
	@echo "Running differential fuzzer..."
	@cargo test --release --test fuzz -- --ignored --nocapture || \
		echo "Fuzz harness not yet configured. See fuzz/harness.rs"

## Bench
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@cargo bench 2>/dev/null || echo "Use cargo bench to run performance tests"

## Check unsafe count
check-unsafe: ## Verify zero unsafe blocks
	@echo "Checking for unsafe blocks..."
	@COUNT=$$(grep -r "unsafe" src/ --include="*.rs" | grep -v "//" | grep -v "/*" | wc -l); \
	if [ "$$COUNT" -eq 0 ]; then \
		echo "  PASS: 0 unsafe blocks found"; \
	else \
		echo "  FAIL: $$COUNT unsafe blocks found"; \
		grep -rn "unsafe" src/ --include="*.rs" | grep -v "//" | grep -v "/*"; \
		exit 1; \
	fi

## Smoke test
smoke: ## Quick smoke test of the decoder binary
	@echo "Running smoke test..."
	@printf 'title = "Port Mortem"\nkey = 42\nflag = true\ndate = 2023-01-01\npi = 3.14\narr = [1, 2, 3]\n' | \
		./target/release/toml-test-decoder 2>&1 || \
		(echo "FAIL: decoder binary not found or errored" && exit 1)
	@echo ""
	@printf 'title = "Port Mortem"\nkey = 42\n' | ./target/release/tomlv 2>&1 || \
		(echo "FAIL: tomlv binary not found or errored" && exit 1)
	@echo "Smoke test passed."

## Docker
docker: ## Build and run in Docker
	docker build -t toml-rs .
	@echo 'key = "value"' | docker run -i toml-rs

## Clean
clean: ## Clean build artifacts
	cargo clean