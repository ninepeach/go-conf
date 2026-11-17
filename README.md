# go-conf

A dependency-free Go parser for structured configuration files.

## Requirements

Go 1.23.2 or newer (as declared in go.mod).

## Usage

```go
package main

import (
    "fmt"
    "log"

    conf "github.com/ninepeach/go-conf"
)

func main() {
    cfg, err := conf.ParseFile("sample.conf")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(cfg["listen"])
}
```

The parser provides `Parse(string)`, `ParseFile(path)`, `ParseWithChecks(string)`, and `ParseFileWithChecks(path)`. Results are `map[string]any`; nested objects are maps, arrays are `[]any`, integers are `int64`, and floats are `float64`. The `WithChecks` variants return token-wrapped values with source-position metadata, rather than plain values.

## Syntax

```conf
listen: 127.0.0.1:8080
name: 'node0'
auth {
  include 'users.conf'
  timeout: 0.5
}
```

Includes are resolved relative to the including file. Recursive include cycles are rejected. Integer suffixes support decimal `k/m/g/t/p/e` and binary `kb/mb/gb/tb/pb/eb` (also `ki/kib`, etc.). Values that overflow `int64` return an error.

Configuration variables can reference previously defined keys or process environment variables. Avoid parsing untrusted configuration files without restricting which files can be included.

## Validation

```sh
go test ./...
go test -race ./...
go vet ./...
```
