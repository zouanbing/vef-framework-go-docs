---
sidebar_position: 9
---

# httpx (Outbound HTTP Client)

`httpx` is the framework's outbound HTTP client with a fluent
request API — construct one `Client` per third-party system and build
per-call `Request`s from it. The integration engine's scoped `http` script
library rides on it; application Go code can use it directly.

> Not to be confused with [`fiberx`](./small-helpers#fiberx), the package of
> Fiber request helpers — the outbound client is `httpx`.

## Quick Start

```go
import "github.com/coldsmirk/vef-framework-go/httpx"

client, err := httpx.New(
    httpx.WithBaseURL("https://api.example.com"),
    httpx.WithTimeout(10*time.Second),
    httpx.WithBearerToken(token),
    httpx.WithRetry(httpx.RetryConfig{}), // defaults: 3 attempts, 100ms→2s backoff
)

var out struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

resp, err := client.NewRequest().
    SetPathParam("id", "42").
    SetQuery("expand", "profile").
    Get(ctx, "/users/:id")
if err != nil {
    return err
}
if !resp.IsSuccess() {
    return fmt.Errorf("upstream returned %s", resp.Status())
}
if err := resp.JSON(&out); err != nil {
    return err
}
```

## Types

| Type | Contract |
| --- | --- |
| `Client` | Immutable HTTP client for calling one upstream service; safe for concurrent use. Construct one per third-party system via `New`. |
| `Option` | Function type `func(*clientConfig)` that customizes client construction. |
| `Request` | Single-use fluent request builder created by `Client.NewRequest`; re-executing one fails with `ErrRequestReused`. |
| `Response` | Fully buffered outcome of a call — the body has been read and the connection released, so it is inert data safe to keep and share. |
| `RetryConfig` | Automatic retry policy configuration, enabled via `WithRetry`. Zero fields fall back to documented defaults. |
| `RequestHook` | Hook type `func(req *Request) error` that runs after a request is fully built and before it is sent. |
| `ResponseHook` | Hook type `func(resp *Response) error` that runs after a response arrives and its body is buffered. |

## Client

`httpx.New(opts ...Option)` validates options eagerly: a malformed base or
proxy URL and conflicting transport-level options fail construction. The
zero-option client is ready to use — no base URL, a 30s call timeout
(retries included), no retries. A `Client` is immutable after `New` and safe
for concurrent use; per-call state lives in the `Request`.

| Option | Behavior |
| --- | --- |
| `WithBaseURL(url)` | absolute URL every relative request URL joins onto; absolute request URLs bypass it |
| `WithTimeout(d)` | bounds a whole call, retries included (default 30s; zero removes the bound) |
| `WithHeader(k, v)` / `WithQuery(k, v)` | default header / query pair on every request |
| `WithBasicAuth(user, pass)` / `WithBearerToken(token)` | default `Authorization` header |
| `WithRetry(cfg)` | enables automatic retries (below) |
| `WithProxy(url)` | outbound proxy |
| `WithTLSConfig(cfg)` | custom TLS configuration |
| `WithCookieJar(jar)` | cookie persistence across calls |
| `WithMaxRedirects(n)` | redirect cap (default 10; zero disables following and returns the 3xx response as-is) |
| `WithMaxResponseBody(n)` | response body byte cap (`ErrResponseTooLarge`); default unlimited |
| `WithRequestHook(hooks...)` | runs after a request is fully built and before it is sent — the hook point for signing, audit, logging; a returned error aborts the call |
| `WithResponseHook(hooks...)` | runs after a response arrives and its body is buffered |
| `WithTransport(rt)` / `WithHTTPClient(hc)` | custom transport / fully custom `http.Client` (mutually exclusive with transport-level options — `ErrConflictingOptions`) |

The client sends `User-Agent: vef/<version>` unless the application sets its
own.

## Hooks

Hook types receive the finalized request or response and return an error to
abort the call:

| Type | Signature | Runs |
| --- | --- | --- |
| `RequestHook` | `func(req *Request) error` | after the request is built, before it is sent |
| `ResponseHook` | `func(resp *Response) error` | after the response arrives and its body is buffered |

A `RequestHook` is the place to sign, audit, or log; it may still mutate
headers. A `ResponseHook` can inspect the buffered response. Multiple hooks
run in registration order.

```go
client, err := httpx.New(
    httpx.WithRequestHook(func(req *httpx.Request) error {
        req.SetHeader("X-Signature", sign(req.Method(), req.Body()))
        return nil
    }),
    httpx.WithResponseHook(func(resp *httpx.Response) error {
        if resp.StatusCode() >= 500 {
            metrics.RecordUpstreamError(resp.StatusCode())
        }
        return nil
    }),
)
```

## Request

`client.NewRequest()` starts a fluent, single-use request builder
(re-executing one fails with `ErrRequestReused`):

| Group | Methods |
| --- | --- |
| Headers | `SetHeader`, `AddHeader`, `SetHeaders` |
| Query | `SetQuery`, `AddQuery`, `SetQueries` |
| Path params | `SetPathParam`, `SetPathParams` — substitute `:name` segments; unresolved segments fail with `ErrMissingPathParam` |
| Cookies / auth | `SetCookie`, `SetBasicAuth`, `SetBearerToken` |
| Body | `SetJSON(v)`, `SetXML(v)`, `SetBody(bytes, contentType)`, `SetBodyReader(r, contentType)`, `SetForm(map)`, `AddFormField(k, v)`, `AddFile(field, path)`, `AddFileReader(field, filename, r)` |
| Timeout | `SetTimeout(d)` — per-request override of the client timeout; zero removes the bound |
| Execute | `Get`, `Post`, `Put`, `Patch`, `Delete`, `Head`, `Options`, or `Do(ctx, method, url)` |
| Introspection | `Method()`, `URL()`, `Header(k)`, `Headers()`, `Body()`, `Context()` — the read surface request hooks use |

`SetForm`/`AddFormField` produce URL-encoded forms; adding files upgrades
the body to multipart automatically.

## Response

| Method | Contract |
| --- | --- |
| `StatusCode()` / `Status()` / `IsSuccess()` | status introspection; `IsSuccess` is 2xx |
| `Header(k)` / `Headers()` / `Cookies()` | response metadata |
| `Body()` / `String()` | the buffered body (always fully read and buffered) |
| `JSON(v)` / `XML(v)` | decode the body |
| `Duration()` | wall time of the call |
| `Attempts()` | attempts made, first call included |
| `Request()` | the originating request |

Non-2xx responses are **not** errors: the call succeeded at the transport
level, and the application decides what statuses mean. Errors are reserved
for transport failures, timeouts, and policy violations.

## Retries

`WithRetry(httpx.RetryConfig{...})` enables automatic retries. Zero fields
resolve to defaults:

| Field | Default | Meaning |
| --- | --- | --- |
| `MaxAttempts` | `3` | total attempts, the first call included |
| `InitialBackoff` | `100ms` | base delay before the first retry; doubles per retry, with full jitter |
| `MaxBackoff` | `2s` | cap on the delay between attempts, a server-sent `Retry-After` included |
| `RetryIf` | — | `func(resp *Response, err error) bool` — custom predicate replacing the default policy entirely. Exactly one of `resp` and `err` is non-nil. |

The default policy retries a transport error (except `context.Canceled`,
`context.DeadlineExceeded`, and `ErrResponseTooLarge`) or a `429`/`502`/`503`/`504`
response, and **only for idempotent methods** (GET, HEAD, PUT, DELETE,
OPTIONS, TRACE) — a POST is never retried unless `RetryIf` allows it.

A streamed request body (`SetBodyReader`) cannot be replayed, so it never
retries even when retries are enabled.

## Testing with a Stub Transport

`WithTransport` is the seam for test doubles, tracing round-trips, and
custom dialing. The test suite uses a small `http.RoundTripper` stub like
this:

```go
type StubTransport struct {
    status int
    body   string
}

func (s *StubTransport) RoundTrip(*http.Request) (*http.Response, error) {
    return &http.Response{
        StatusCode: s.status,
        Status:     fmt.Sprintf("%d %s", s.status, http.StatusText(s.status)),
        Header:     make(http.Header),
        Body:       io.NopCloser(strings.NewReader(s.body)),
    }, nil
}
```

Wire it into a client and every request returns the canned response without
leaving the process:

```go
client, _ := httpx.New(
    httpx.WithTransport(&StubTransport{status: http.StatusTeapot, body: "stubbed"}),
)

resp, err := client.NewRequest().Get(ctx, "http://example.test/anything")
```

## Error Sentinels

| Error | Trigger |
| --- | --- |
| `ErrInvalidOption` | malformed base/proxy URL or other invalid option value |
| `ErrConflictingOptions` | `WithHTTPClient` combined with transport-level options; `WithTransport` combined with `WithProxy` or `WithTLSConfig` |
| `ErrInvalidRequestURL` | unparsable request URL, or a relative URL without a base URL |
| `ErrMissingPathParam` | a `:name` segment left unresolved |
| `ErrRequestReused` | second execution of a single-use request |
| `ErrTooManyRedirects` | redirect cap exceeded |
| `ErrResponseTooLarge` | response body over the configured cap |

## See also

- [Integration Engine](../integration/overview) — systems configure `httpx` clients declaratively (auth schemes, retry policy, timeouts)
- [Small Helpers](./small-helpers) — `fiberx`, the inbound Fiber request helpers formerly named `httpx`
