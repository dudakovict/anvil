// Package validate provides support for validating structs.
package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	entranslations "github.com/go-playground/validator/v10/translations/en"
)

// The validator is safe for concurrent use and caches struct metadata, so a
// single package-level instance backs all Check calls.
var (
	validate *validator.Validate
	trans    ut.Translator
)

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())

	validate.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" {
			name, _, _ = strings.Cut(f.Tag.Get("form"), ",")
		}

		if name == "-" {
			return ""
		}

		return name
	})

	locale := en.New()
	trans, _ = ut.New(locale, locale).GetTranslator("en")

	if err := entranslations.RegisterDefaultTranslations(validate, trans); err != nil {
		panic(fmt.Sprintf("registering validator translations: %s", err))
	}
}

func Check(v any) error {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}

	fields := make(FieldErrors, 0, len(verrs))
	for _, fe := range verrs {
		fields = append(fields, FieldError{
			Field: fe.Field(),
			Err:   fe.Translate(trans),
		})
	}

	return fields
}
