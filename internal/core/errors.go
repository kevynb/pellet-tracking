package core

import (
	"errors"
	"fmt"
	"strings"
)

// Domain errors returned by core operations.
var (
	ErrBrandNotFound       = errors.New("brand not found")
	ErrBrandInUse          = errors.New("brand is referenced by purchases or consumptions")
	ErrPurchaseNotFound    = errors.New("purchase not found")
	ErrConsumptionNotFound = errors.New("consumption not found")
)

// ValidationError describes an invalid field with an associated message.
type ValidationError struct {
	Field   string
	Message string
}

// ValidationErrors aggregates validation errors for operations.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return "validation failed"
	}
	parts := make([]string, len(v))
	for i, ve := range v {
		if ve.Field != "" {
			parts[i] = fmt.Sprintf("%s: %s", ve.Field, ve.Message)
			continue
		}
		parts[i] = ve.Message
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(parts, ", "))
}

// Has reports whether the provided field has a validation error.
func (v ValidationErrors) Has(field string) bool {
	for _, ve := range v {
		if ve.Field == field {
			return true
		}
	}
	return false
}

// AppendIf adds the error when the condition is true, returning the new slice.
func (v ValidationErrors) AppendIf(cond bool, field, message string) ValidationErrors {
	if cond {
		v = append(v, ValidationError{Field: field, Message: message})
	}
	return v
}
