# TOML Validator

If Go is installed, it's simple to try it out:

```bash
go get github.com/BurntSushi/toml/tomlv
tomlv some-toml-file.toml
```

At the moment, only one error message is reported at a time. Error messages
are included. No output means that the files given are valid TOML, or there
is a bug in `tomlv`.

Compatible with [f68d014bfd](https://github.com/mojombo/toml/commit/f68d014bfd4a84a64fb5f6a7c1a83a4162415d4b).


