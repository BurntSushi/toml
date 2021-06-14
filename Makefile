# toml: TOML array element cannot contain a table
# Dotted keys are not supported yet.
SKIP_ENCODE?=valid/inline-table-nest,valid/key-dotted

# Dotted keys are not supported yet.
SKIP_DECODE=valid/key-dotted

# No easy way to see if this was a datetime or local datetime; we should extend
# meta with new types for this, which seems like a good idea in any case.
SKIP_DECODE+=,valid/datetime-local-date,valid/datetime-local-time,valid/datetime-local
SKIP_ENCODE+=,valid/datetime-local-date,valid/datetime-local-time,valid/datetime-local

all:
	@e=0  # So it won't stop on the first command that fails.
	@go install ./...
	@go test ./... || e=1
	@toml-test -skip="${SKIP_DECODE}" toml-test-decoder || e=1
	@toml-test -encoder -skip="${SKIP_ENCODE}" toml-test-encoder || e=1
	@exit $e
