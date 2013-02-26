# TOML parser for Go with reflection

TOML stands for Tom's Obvious, Minimal Language.

Spec: https://github.com/mojombo/toml

Documentation: http://godoc.org/github.com/BurntSushi/toml

Installation:

```bash
go get github.com/BurntSushi/toml
```

Try the toml checker:

```bash
go get github.com/BurntSushi/toml/tomlcheck
tomlcheck some-toml-file.toml
```

## Examples

This package works similarly to how the Go standard library handles `XML`
and `JSON`. Namely, data is loaded into Go values via reflection.

For the simplest example, consider some TOML file as just a list of keys
and values:

```toml
Age = 25
Cats = [ "Cauchy", "Plato" ]
Pi = 3.14
Perfection = [ 6, 28, 496, 8128 ]
DOB = 1987-07-05T05:45:00Z
```

Which could defined in Go as:

```go
type Config struct {
  Age int
  Cats []string
  Pi float64
  Perfection []int
  DOB time.Time // requires `import time`
}
```

And then decoded with:

```go
var conf Config
if err := toml.Decode(tomlData, &conf); err != nil {
  // handle error
}
```

