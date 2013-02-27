# TOML Validator

If Go is installed, it's simple to try it out:

```bash
go get github.com/BurntSushi/toml/tomlv
tomlv some-toml-file.toml
```

At the moment, only one error message is reported at a time. Error messages
include line numbers. No output means that the files given are valid TOML, or 
there is a bug in `tomlv`.

Compatible with TOML commit
[3f4224ecdc](https://github.com/mojombo/toml/commit/3f4224ecdc4a65fdd28b4fb70d46f4c0bd3700aa).


