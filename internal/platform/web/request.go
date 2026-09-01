package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/form/v4"

	"github.com/dudakovict/anvil/internal/platform/validate"
)

type validator interface {
	Validate() error
}

// queryDecoder is safe for concurrent use and caches struct metadata.
var queryDecoder = form.NewDecoder()

// Decode unmarshals the body into T and validates it, by its validate tags
// or by its own Validate method when implemented.
func Decode[T any](r *http.Request) (T, error) {
	var v T

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&v); err != nil {
		return v, err
	}

	if err := check(v); err != nil {
		return v, err
	}

	return v, nil
}

// DecodeQuery maps query parameters into T by its form tags and validates it
// the same way Decode does.
func DecodeQuery[T any](r *http.Request) (T, error) {
	var v T

	if err := queryDecoder.Decode(&v, r.URL.Query()); err != nil {
		return v, err
	}

	if err := check(v); err != nil {
		return v, err
	}

	return v, nil
}

func check(v any) error {
	if val, ok := v.(validator); ok {
		return val.Validate()
	}

	return validate.Check(v)
}
