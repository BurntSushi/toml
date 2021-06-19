all:
	@e=0  # So it won't stop on the first command that fails.
	@go install ./...
	@go test ./... || e=1
	@exit $e
