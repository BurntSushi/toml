install:
	go install ./...

test: install
	go test -v
	toml-test-decoder < ./_examples/example.toml > /dev/null || test $$? -eq 0
	toml-test-decoder < ./_examples/invalid.toml &> /dev/null || test $$? -eq 1
	toml-test-decoder < ./_examples/example.toml | toml-test-encoder > /dev/null || test $$? -eq 0
	tomlv ./_examples/example.toml || test $$? -eq 0
	tomlv ./_examples/invalid.toml &> /dev/null || test $$? -eq 1

fmt:
	gofmt -w *.go */*.go
	colcheck *.go */*.go

tags:
	find ./ -name '*.go' -print0 | xargs -0 gotags > TAGS

push:
	git push origin master
	git push github master
