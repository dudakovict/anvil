---
name: add-adapter
description: Add a driven adapter for an external API (payment provider, exchange rates, another internal service) behind an out-port, using platform/httpclient. Use when the core needs to call an external system.
---

# Adding an external API adapter

The core never learns HTTP is involved: it depends on an out-port interface,
the adapter implements it with `platform/httpclient`, main wires them. The
example integrates an exchange-rate API as `adapter/hnb`.

## 1. Out-port — `internal/core/port/rate.go`

Defined by the consumer, in domain terms (no HTTP types):

```go
type RateProvider interface {
	Rate(ctx context.Context, currency string) (domain.Rate, error)
}
```

Append the interface to the mockgen line in the Makefile and run `make mocks`.
Domain types/errors it returns live in `core/domain`.

## 2. Adapter — `internal/adapter/hnb/hnb.go`

Package named after the external system. One `httpclient.Client` per upstream,
created in the constructor:

```go
type Client struct {
	hc      *httpclient.Client
	baseURL string
}

var _ port.RateProvider = (*Client)(nil)

func New(baseURL string) *Client {
	return &Client{
		hc:      httpclient.New(httpclient.WithTracing()),
		baseURL: baseURL,
	}
}
```

- `WithTracing()` gives a client span + traceparent propagation;
  `WithLogging(body)` logs method/host/url/status/duration, body=true adds
  both bodies so keep it false when payloads carry secrets or PII;
  `WithTimeout` if 30s default does not fit
- Requests via `c.hc.Do(ctx, &httpclient.Request{Method, URL, Body, Header})`;
  check `resp.OK()`, decode with `resp.JSON(&dto)`
- The adapter owns its wire DTOs (json tags) and converts them to domain
  types — same layering as `postgres/model.go`
- Map upstream failures to domain errors (e.g. 404 → `domain.ErrRateNotFound`);
  wrap the rest with context: `fmt.Errorf("fetching rate: %w", err)`

## 3. Config + wiring

- Add the base URL (and credentials) to `internal/config` under a new
  `envPrefix` block + `.env.example` + README config table
- `cmd/api/main.go`: construct the adapter, pass it into the service
  constructor that needs it (the service takes the port, never the adapter
  type)

## 4. Tests

Service tests mock the port with gomock (see the `go-tests` skill) — no HTTP
in core tests. If the adapter itself needs tests, use `httptest.NewServer`
with canned responses and point `baseURL` at it.

## Verify

`make mocks && make fmt && make lint && make test`; run the app and confirm
the outbound call shows as a client span inside the request trace in Grafana.
