// Package httpclient provides support for calling external APIs.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const defaultTimeout = 30 * time.Second

type Option func(*Client)

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithTransport applies mutate to a clone of http.DefaultTransport.
func WithTransport(mutate func(*http.Transport)) Option {
	return func(c *Client) {
		t := http.DefaultTransport.(*http.Transport).Clone()
		mutate(t)
		c.transport = t
	}
}

// WithLogging is off by default so secret-bearing clients never log.
// body includes the request and response bodies, keep it off for payloads
// with secrets or PII.
func WithLogging(body bool) Option {
	return func(c *Client) {
		c.logging = true
		c.logBody = body
	}
}

// WithTracing emits a client span per request and propagates traceparent.
func WithTracing() Option {
	return func(c *Client) { c.tracing = true }
}

type Client struct {
	hc *http.Client

	timeout   time.Duration
	transport http.RoundTripper
	logging   bool
	logBody   bool
	tracing   bool
}

// New builds a Client intended to be reused per upstream service.
func New(opts ...Option) *Client {
	c := Client{
		timeout: defaultTimeout,
	}

	for _, opt := range opts {
		opt(&c)
	}

	if c.transport == nil {
		c.transport = http.DefaultTransport
	}

	if c.logging {
		c.transport = &loggingRoundTripper{
			base: c.transport,
			body: c.logBody,
		}
	}

	if c.tracing {
		c.transport = otelhttp.NewTransport(c.transport)
	}

	c.hc = &http.Client{
		Timeout:   c.timeout,
		Transport: c.transport,
	}

	return &c
}

type Request struct {
	Method string
	URL    string
	Body   []byte
	Header map[string]string
}

type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (c *Client) Do(ctx context.Context, r *Request) (*Response, error) {
	var body io.Reader

	if r.Body != nil {
		body = bytes.NewReader(r.Body)
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	for k, v := range r.Header {
		req.Header.Set(k, v)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       b,
		Header:     resp.Header,
	}, nil
}

func (r *Response) OK() bool {
	return r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices
}

func (r *Response) JSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

type loggingRoundTripper struct {
	base http.RoundTripper
	body bool
}

func (l *loggingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	reqAttrs := []any{
		"method", r.Method,
		"host", r.URL.Host,
		"url", r.URL.String(),
	}

	if l.body && r.GetBody != nil {
		b, err := r.GetBody()
		if err != nil {
			return nil, fmt.Errorf("get body: %w", err)
		}

		defer func() { _ = b.Close() }()

		reqBody, err := io.ReadAll(b)
		if err != nil {
			return nil, fmt.Errorf("read all: %w", err)
		}

		reqAttrs = append(reqAttrs, "body", string(reqBody))
	}

	slog.InfoContext(r.Context(), "http request", reqAttrs...)

	start := time.Now()

	resp, err := l.base.RoundTrip(r)
	if err != nil {
		slog.ErrorContext(r.Context(), "http request failed",
			"host", r.URL.Host,
			"url", r.URL.String(),
			"duration_s", time.Since(start).Seconds(),
			"err", err,
		)

		return nil, fmt.Errorf("round trip: %w", err)
	}

	respAttrs := []any{
		"host", r.URL.Host,
		"status", resp.StatusCode,
		"duration_s", time.Since(start).Seconds(),
	}

	if l.body {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read all: %w", err)
		}

		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		respAttrs = append(respAttrs, "body", string(respBody))
	}

	slog.InfoContext(r.Context(), "http response", respAttrs...)

	return resp, nil
}
