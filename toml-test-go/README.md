# Implements the TOML test suite interface

This is an implementation of the interface expected by
[toml-test](https://github.com/BurntSushi/toml-test) for my
[toml parser written in Go](https://github.com/BurntSushi/toml).
In particular, it maps TOML data on `stdin` to a JSON format on `stdout`.


Compatible with TOML commit
[00682c6](https://github.com/mojombo/toml/commit/00682c6877466d4031b4f01c5a2182b557227690)

Compatible with `toml-test` commit
[277dd02](https://github.com/BurntSushi/toml-test/commit/277dd02e2e0d9158706a07d16c048c1008a5cb5f)

