// Package validator wraps go-playground/validator behind a shared instance.
package validator

import (
	"github.com/go-playground/validator/v10"
)

// V is the shared validator instance.
var V = validator.New()
