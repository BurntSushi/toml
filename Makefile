TOML_TESTDIR?=tests

install:
	go install ./...

test: install
	go test -v ./...
	toml-test -testdir="${TOML_TESTDIR}"          toml-test-decoder
	toml-test -testdir="${TOML_TESTDIR}" -encoder toml-test-encoder

fmt:
	gofmt -w *.go */*.go
	colcheck *.go */*.go

tags:
	find ./ -name '*.go' -print0 | xargs -0 gotags > TAGS

push:
	git push origin master
	git push github master

