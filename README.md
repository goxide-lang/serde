# Goxide serde

Format-independent serialization protocols and a JSON implementation for Go 1.27. The root `github.com/goxide-lang/serde` package provides `Serializer`, `Deserializer`, `Visitor`, container protocols and structured errors. `github.com/goxide-lang/serde/json` implements JSON; its Go package name is `serdejson`.

## Use

After serde v0.0.1 is published, install it with:

```sh
go get github.com/goxide-lang/serde@v0.0.1
```

```go
package main

import (
    "fmt"
    "github.com/goxide-lang/serde"
    serdejson "github.com/goxide-lang/serde/json"
)

func main() {
    data, err := serdejson.Encode(func(s serde.Serializer) error { return s.String("hello") })
    if err != nil { panic(err) }
    value, err := serdejson.Unmarshal(data, serde.ReadString)
    if err != nil { panic(err) }
    fmt.Printf("%s %s\n", data, value) // "hello" hello
}
```

Goxide programs can use `#[derive(serde::Serialize, serde::Deserialize)]` after `use serde;`. Use the serde version pinned by the compiler; see the compiler's [JSON guide](https://github.com/goxide-lang/goxide/blob/main/docs/serde-json.md) and [current capability table](https://github.com/goxide-lang/goxide/blob/main/docs/mvp-gap-audit.md).

## JSON behavior

- Booleans, signed and unsigned 64-bit integers, finite floating-point values, strings, bytes, optional values, sequences, string-keyed maps and structs are supported. Bytes use arrays of numbers, not base64 strings.
- The variant protocol supports unit and single-payload variants. Goxide's compiler supplies generated callbacks for supported enum shapes; its current wire-mapping limits are listed in the capability table.
- Decoding preserves integer precision, rejects duplicate object keys, invalid UTF-8, unpaired Unicode surrogates and trailing values, and checks skipped values too. Errors retain their kind, path and underlying cause where available.
- Input and output are limited to 16 MiB, with a nesting limit of 128. Returned data and decoded strings or bytes are owned values.

The JSON API processes complete byte slices. Streaming I/O, a generic `Value` / `Number` / `RawValue` tree, PATCH-style three-state fields, base64 adapters and YAML are not implemented. Tuple-variant and struct-variant methods are not part of the protocol.

## Development

```sh
GOWORK=off GOTOOLCHAIN=local go test -race ./...
GOWORK=off GOTOOLCHAIN=local go vet ./...
```

Licensed under [Mozilla Public License 2.0](LICENSE).
