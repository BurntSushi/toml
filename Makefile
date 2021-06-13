TOML_TESTDIR?=tests

# TODO: these should be fixed
SKIP_DECODE?=valid/array-table-array-string-backslash,\
	     valid/inline-table-array,\
	     valid/inline-table,\
	     valid/nested-inline-table-array,\
	     invalid/bad-utf8,\
	     invalid/key-multiline,\
	     invalid/inline-table-empty,\
	     invalid/inline-table-nest,\
	     valid/inline-table-empty,\
	     valid/inline-table-nest

SKIP_ENCODE?=valid/inline-table-array,\
	     valid/inline-table,\
	     valid/nested-inline-table-array,\
	     valid/array-table-array-string-backslash,\
	     valid/inline-table-empty,\
	     valid/inline-table-nest,\
	     valid/key-escapes

install:
	@go install ./...

test: install
	@go test ./...
	@toml-test -testdir="${TOML_TESTDIR}" -skip="${SKIP_DECODE}"          toml-test-decoder
	@toml-test -testdir="${TOML_TESTDIR}" -skip="${SKIP_ENCODE}" -encoder toml-test-encoder

fmt:
	gofmt -w *.go */*.go
	colcheck *.go */*.go

tags:
	find ./ -name '*.go' -print0 | xargs -0 gotags > TAGS

push:
	git push origin master
	git push github master

