package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestV(t *testing.T) {
	type input struct {
		Name  string `validate:"required"`
		Email string `validate:"omitempty,email"`
	}

	assert.NoError(t, V.Struct(input{Name: "ok", Email: "a@b.c"}))
	assert.Error(t, V.Struct(input{Name: ""}))
	assert.Error(t, V.Struct(input{Name: "ok", Email: "not-an-email"}))
}
