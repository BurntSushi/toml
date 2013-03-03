# Implements the TOML test suite interface

This is an implementation of the interface expected by
[toml-test](https://github.com/BurntSushi/toml-test) for my
[toml parser written in Go](https://github.com/BurntSushi/toml).
In particular, it maps TOML data on `stdin` to a JSON format on `stdout`.


Compatible with TOML commit
[00682c6](https://github.com/mojombo/toml/commit/00682c6877466d4031b4f01c5a2182b557227690)

Compatible with `toml-test` commit
[7f83b06](https://github.com/BurntSushi/toml-test/commit/7f83b065a165e77696b949985f6cc923e87d3e38)

