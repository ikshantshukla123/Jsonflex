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
through verbatim.

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
- Conversion works on a decode → walk → re-encode basis using the standard
  library, so it is correct for all valid JSON. It is not zero-allocation; a
  byte-level key rewriter is planned (see the roadmap) for hot paths.

## Roadmap

- **v1** (this release) — bidirectional net/http middleware, recursive
  nested/array support, configurable exclusions, tests & benchmarks.
- **v2** — framework adapters (Gin, Echo, Chi, Fiber).
- **v3** — optional streaming transformation for large payloads.
- **v4** — byte-level key rewriter that copies value bytes verbatim for
  near-zero allocations on hot paths.

## Benchmarks

Indicative numbers on an Apple Silicon laptop (`go test -bench=.`); run them on
your own hardware for accurate figures:

```
BenchmarkToSnakeCase     ~7.8 µs/op   ~56 MB/s    (mixed nested payload)
BenchmarkCamelToSnake    ~136 ns/op   1 alloc/op  (single key)
BenchmarkSnakeToCamel    ~111 ns/op   1 alloc/op  (single key)
```

## License

MIT — see [LICENSE](LICENSE).
