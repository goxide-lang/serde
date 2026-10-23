# Goxide serde

Format-independent serialization protocols and a JSON implementation for Go 1.27. The root `github.com/goxide-lang/serde` package provides `Serializer`, `Deserializer`, `Visitor`, container protocols and structured errors. `github.com/goxide-lang/serde/json` implements JSON; its Go package name is `serdejson`.

## Use

Install the version selected by your application or compiler:

```sh
go get github.com/goxide-lang/serde
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

Goxide programs use ordinary `Serialize` and `Deserialize` traits. The independent `serde::derive` procedural macro crate generates their implementations:

```goxide
package main
use serde;
use serde::json as json;

#[derive(serde::derive::Serialize, serde::derive::Deserialize)]
struct Config { Port int }

func main() {
    config, err := json::Unmarshal([]byte("{\"Port\":8080}"), serde::Read[Config])
    if err != nil { panic(err) }
    data, err := json::Marshal(&config, serde::Write[Config])
    if err != nil { panic(err) }
    println(string(data))
}
```

Use the serde and std versions pinned by the compiler. `serde::Read[T]` and `serde::Write[T]` are ordinary generic functions and can be passed as callbacks. Derives require the corresponding protocol for each field or payload; serialization and deserialization do not require one another. `#[serde(rename="name")]`, `#[serde(default)]`, `#[serde(skip)]` and container `#[serde(deny_unknown_fields)]` control struct fields. Deserialization defaults and skipped fields call the ordinary `std::Default` implementation in declaration order, after input and required fields have been validated.

## JSON behavior

- Booleans, signed and unsigned 64-bit integers, finite floating-point values, strings, bytes, optional values, sequences, string-keyed maps and structs are supported. Bytes use arrays of numbers, not base64 strings.
- The variant protocol supports unit and single-payload variants. The derive macros implement ordinary enum protocols. Unit variants can use string renames or complete explicit signed-int64 numeric mappings; numeric mappings cannot mix with renames. Invalid enum values return structured errors.
- Decoding preserves integer precision, rejects duplicate object keys, invalid UTF-8, unpaired Unicode surrogates and trailing values, and checks skipped values too. Errors retain their kind, path and underlying cause where available.
- Input and output are limited to 16 MiB, with a nesting limit of 128. Returned data and decoded strings or bytes are owned values.

The JSON API processes complete byte slices. Streaming I/O, a generic `Value` / `Number` / `RawValue` tree, PATCH-style three-state fields, base64 adapters and YAML are not implemented. Tuple-variant and struct-variant methods are not part of the protocol.

## Development

```sh
GOWORK=off GOTOOLCHAIN=local go test -race ./...
GOWORK=off GOTOOLCHAIN=local go vet ./...
```

Licensed under [Mozilla Public License 2.0](LICENSE).
