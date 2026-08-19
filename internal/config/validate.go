package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

const fieldErrMsg = "Key '%s': Field validation for '%s' failed on '%s(%s)'"

func validate(cfg *Configuration) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name, _, _ := strings.Cut(fld.Tag.Get("yaml"), ",")
		if name == "-" {
			return ""
		}
		return name
	})

	err := validate.Struct(cfg)
	if err == nil {
		return nil
	}

	fieldErrors, ok := errors.AsType[validator.ValidationErrors](err)
	if !ok {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	var sb strings.Builder
	for _, fe := range fieldErrors {
		if sb.Len() == 0 {
			sb.WriteString("invalid configuration:")
		}
		sb.WriteString("\n  ")
		fmt.Fprintf(&sb, fieldErrMsg, fe.Namespace(), fe.Field(), fe.Tag(),
			fe.Param())
	}
	return errors.New(sb.String())
}
