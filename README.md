# jsonflex

Transparent HTTP middleware that converts JSON key naming between **camelCase**
(what your JavaScript/TypeScript frontend sends) and **snake_case** (what your
Go structs and database expect) — in both directions, automatically.

Keep idiomatic Go structs on the server and idiomatic camelCase on the client
without writing duplicate DTOs or hand-maintaining `json:"..."` tags on every
field.

```go
mux := http.NewServeMux()
mux.Handle("/api/", apiHandler)

// One line: request bodies camelCase -> snake_case, responses snake_case -> camelCase.
http.ListenAndServe(":8080", jsonflex.Middleware()(mux))
```

## Install

```sh
go get github.com/ikshantshukla123/jsonflex
```

Requires Go 1.22+.

## What it does

Given this request from a browser:

```json
{ "userName": "amy", "homeAddress": { "zipCode": "10001" } }
```

your handler reads plain snake_case, so ordinary struct tags just work:

```go
var in struct {
    UserName    string `json:"user_name"`
    HomeAddress struct {
        ZipCode string `json:"zip_code"`
    } `json:"home_address"`
}
json.NewDecoder(r.Body).Decode(&in)
```

and whatever snake_case JSON your handler writes goes back to the client as
camelCase.

- **Recursive** — nested objects and arrays are converted to any depth.
- **Lossless values** — integers keep full precision (no `float64` rounding),
  and `<`, `>`, `&` are not HTML-escaped.
- **Safe by default** — only `application/json` (and `+json`) bodies are
  touched; empty or invalid JSON is passed through unchanged instead of
  erroring.

## Options

```go
jsonflex.Middleware(
    jsonflex.WithRequestConversion(true),   // incoming camelCase -> snake_case (default on)
    jsonflex.WithResponseConversion(true),  // outgoing snake_case -> camelCase (default on)
    jsonflex.Exclude("rawMeta", "payload"), // leave these keys and their subtrees untouched
    jsonflex.WithRequestKeyFunc(customFn),  // override the request transform
    jsonflex.WithResponseKeyFunc(customFn), // override the response transform
)
```

`Exclude` is for fields whose value is arbitrary, caller-controlled JSON that
you don't want reshaped — the key keeps its name and the whole subtree is passed
through verbatim. Matching is direction-agnostic: `Exclude("rawMeta")` protects
the field on both the request side (`rawMeta`) and the response side
(`raw_meta`), and passing either case form is equivalent.

## Framework adapters

The core middleware is standard `net/http`, so it works directly with any
`net/http`-compatible router. Thin adapter modules are provided for the popular
frameworks — each keeps its framework dependency out of the core module, so
importing jsonflex never pulls in Gin/Echo/Chi.

**Chi** (and `net/http`, `chi`, `gorilla/mux`, stdlib `ServeMux`) — works directly:

```go
r := chi.NewRouter()
r.Use(jsonflex.Middleware())
```

**Gin**:

```go
import ginflex "github.com/ikshantshukla123/jsonflex/adapters/gin"

r := gin.New()
r.Use(ginflex.Middleware())
```

**Echo**:

```go
import echoflex "github.com/ikshantshukla123/jsonflex/adapters/echo"

e := echo.New()
e.Use(echoflex.Middleware())
```

Every adapter accepts the same `jsonflex.Option` values as the core middleware
(`Exclude`, `WithResponseConversion(false)`, custom key funcs, …). Install the
one you need:

```sh
go get github.com/ikshantshukla123/jsonflex/adapters/gin
go get github.com/ikshantshukla123/jsonflex/adapters/echo
# Chi needs no adapter; go get the core module and use jsonflex.Middleware()
```

## Standalone conversion

The transforms are usable without the middleware:

```go
out, err := jsonflex.ToSnakeCase([]byte(`{"userName":"amy"}`))
// out == {"user_name":"amy"}

out, err = jsonflex.ToCamelCase([]byte(`{"user_name":"amy"}`))
// out == {"userName":"amy"}

jsonflex.CamelToSnake("APIKey") // "api_key"
jsonflex.SnakeToCamel("user_id") // "userId"
```

## Notes & caveats

- **Acronyms don't perfectly round-trip.** `CamelToSnake("userID")` is
  `"user_id"`, and `SnakeToCamel("user_id")` is `"userId"` (not `"userID"`).
  This matches conventional JavaScript naming. Use `Exclude` for keys that must
  be preserved exactly.
- **Response conversion buffers the body** so it can be re-encoded. For
  streaming endpoints (e.g. Server-Sent Events) disable it with
  `WithResponseConversion(false)`.

## How it works

For the built-in camelCase/snake_case directions, jsonflex uses a **byte-level
engine**: it scans the raw JSON, copies every value and structural token
verbatim, and rewrites only object keys. Values are never decoded, so integer
precision and byte content are preserved exactly, key order is preserved, and a
typical body converts with a **single allocation** (the output buffer) instead
of one per element.

Supplying a custom `WithRequestKeyFunc` / `WithResponseKeyFunc` falls back to a
decode → walk → re-encode engine, which is slower but supports arbitrary naming
schemes. Both engines are correctness-tested against each other.

## Roadmap

- **v1** — bidirectional net/http middleware, recursive nested/array support,
  configurable exclusions, tests & benchmarks.
- **v1.1** — reusable `Converter` API (`New`, `ConvertRequestBody`,
  `ConvertResponseBody`) shared by the middleware and adapters.
- **v2** — framework adapters for Gin, Echo, and Chi, each an independent
  module so the core stays dependency-free.
- **v3 (done, shipped as v1.3)** — byte-level key rewriter that copies value
  bytes verbatim: ~9× faster and ~1 allocation per body versus the tree-walk
  engine.
- **v4** — optional streaming transformation for very large payloads; Fiber
  adapter.

## Benchmarks

Indicative numbers on an Apple Silicon laptop (`go test -bench=.`); run them on
your own hardware for accurate figures. Byte-level engine vs. tree-walk on the
same mixed nested payload:

```
BenchmarkFastPathToSnake   ~0.87 µs/op   ~510 MB/s     1 alloc/op   (byte-level, default)
BenchmarkTreeWalkToSnake   ~7.7  µs/op   ~57  MB/s   150 allocs/op  (custom KeyFunc path)
BenchmarkCamelToSnake      ~136  ns/op                 1 alloc/op   (single key)
BenchmarkSnakeToCamel      ~111  ns/op                 1 alloc/op   (single key)
```

## License

MIT — see [LICENSE](LICENSE).
