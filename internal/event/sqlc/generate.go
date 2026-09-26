// Package sqlc contains the sqlc-generated queries and the go:generate hook for gomock.
package sqlc

//go:generate go run go.uber.org/mock/mockgen -source=querier.go -package=mock -destination=mock/querier.go Querier
