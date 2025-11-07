package graph

import (
	"context"
	"net/http"

	"github.com/graphql-go/graphql"
)

type GraphContext struct {
	Schema       *graphql.Schema
	Pretty       bool
	GraphiQL     bool
	Playground   bool
	DEBUG        bool
	RootObjectFn func(ctx context.Context, r *http.Request) map[string]interface{}

	// Optional: Custom token extraction from request
	// If not provided, default Bearer token extraction will be used
	TokenExtractorFn func(*http.Request) string

	// Optional: Custom user details fetching based on token
	// If not provided, user details will not be added to rootValue
	UserDetailsFn func(token string) (interface{}, error)

	// Optional: Enable query validation (depth, complexity, introspection checks)
	// Default: false (validation disabled)
	EnableValidation bool

	// Optional: Enable response sanitization (removes field suggestions from errors)
	// Default: false (sanitization disabled)
	EnableSanitization bool
}
