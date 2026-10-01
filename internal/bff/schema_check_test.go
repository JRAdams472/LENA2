package bff

import (
	"testing"

	"github.com/graph-gophers/graphql-go"
)

func TestSchemaParses(t *testing.T) {
	if _, err := graphql.ParseSchema(schema, &Resolver{}); err != nil {
		t.Fatal(err)
	}
}
