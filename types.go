package graph

import (
	"context"
	"net/http"

	"github.com/graphql-go/graphql"
)

// GraphContext configures a GraphQL handler with schema, authentication, and security settings.
//
// Schema Configuration (choose one):
//   - Schema: Use a pre-built graphql.Schema
//   - SchemaParams: Use the builder pattern with QueryFields and MutationFields
//   - Neither: A default "hello world" schema will be created
//
// Security Modes:
//   - DEBUG mode (DEBUG: true): Disables all validation and sanitization for development
//   - Production mode (DEBUG: false): Enables validation and sanitization based on configuration flags
//
// Authentication:
//   - TokenExtractorFn: Extract tokens from requests (defaults to Bearer token extraction)
//   - UserDetailsFn: Fetch user details from the extracted token
//   - RootObjectFn: Custom root object setup for advanced use cases
//
// Example Development Setup:
//
//	ctx := &graph.GraphContext{
//	    SchemaParams: &graph.SchemaBuilderParams{
//	        QueryFields: []graph.QueryField{getUserQuery()},
//	    },
//	    DEBUG:      true,
//	    Playground: true,
//	}
//
// Example Production Setup:
//
//	ctx := &graph.GraphContext{
//	    SchemaParams:       &graph.SchemaBuilderParams{...},
//	    DEBUG:              false,
//	    EnableValidation:   true,  // Max depth: 10, Max aliases: 4, Max complexity: 200
//	    EnableSanitization: true,  // Remove field suggestions from errors
//	    Playground:         false,
//	    UserDetailsFn: func(token string) (interface{}, error) {
//	        return validateJWT(token)
//	    },
//	}
type GraphContext struct {
	// Schema: Provide either Schema OR SchemaParams (not both)
	// If both are nil, a default "hello world" schema will be created
	Schema *graphql.Schema

	// SchemaParams: Alternative to Schema - will be built automatically
	// If nil and Schema is also nil, defaults to hello world query/mutation
	SchemaParams *SchemaBuilderParams

	// Pretty: Pretty-print JSON responses
	Pretty bool

	// GraphiQL: Enable GraphiQL interface (deprecated, use Playground instead)
	GraphiQL bool

	// Playground: Enable GraphQL Playground interface
	Playground bool

	// DEBUG mode skips validation and sanitization for easier development
	// Default: false (validation enabled)
	DEBUG bool

	// RootObjectFn: Custom function to set up root object for each request
	// Called before token extraction and user details fetching
	RootObjectFn func(ctx context.Context, r *http.Request) map[string]interface{}

	// TokenExtractorFn: Custom token extraction from request
	// If not provided, default Bearer token extraction will be used
	TokenExtractorFn func(*http.Request) string

	// UserDetailsFn: Custom user details fetching based on token
	// If not provided, user details will not be added to rootValue
	// The details are accessible in resolvers via GetRootInfo(p, "details", &user)
	UserDetailsFn func(token string) (interface{}, error)

	// EnableValidation: Enable query validation (depth, complexity, introspection checks)
	// Default: false (validation disabled)
	// When enabled: Max depth=10, Max aliases=4, Max complexity=200, Introspection blocked
	EnableValidation bool

	// EnableSanitization: Enable response sanitization (removes field suggestions from errors)
	// Default: false (sanitization disabled)
	// Prevents information disclosure by removing "Did you mean X?" suggestions
	EnableSanitization bool
}

type ResolveParams graphql.ResolveParams
