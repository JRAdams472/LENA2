package sqlc

//go:generate go run go.uber.org/mock/mockgen -source=querier.go -package=mock -destination=mock/querier.go Querier
